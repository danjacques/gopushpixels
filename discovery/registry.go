// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package discovery

import (
	"sync"
	"time"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"
)

type discoveredDeviceState struct {
	dh      *protocol.DiscoveryHeaders
	expires time.Time
}

// Registry tracks a list of discovered headers, instantiating a device.D
// instance for each unique device. It uses device.Remote instances.
//
// If successive headers are observed for the same device, Registry will update
// the device's header values.
//
// Registry automatically expires devices if they haven't been observed within
// its Expiration threshold. When a device is expired, it will have its DoneC
// channel closed, marking it done.
//
// Registry is safe for concurrent use.
type Registry struct {
	// Expiration is the amount of time after which a device is considered
	// to no longer exist.
	//
	// If <= 0, a device will never expire once observed.
	Expiration time.Duration

	// DeviceRegistry, if not nil, is a device.Registry that will be updated when
	// new devices are registered.
	DeviceRegistry *device.Registry

	// Protects the following data members.
	mu sync.Mutex
	// Map of active devices.
	devices map[string]*registryEntry
}

// Shutdown shuts down Registry monitoring and all managed devices. It blocks
// until everything is terminated.
func (reg *Registry) Shutdown() {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	for _, e := range reg.devices {
		reg.unregisterEntryLocked(e)
	}
}

// Devices returns the list of current devices, in no particular order.
func (reg *Registry) Devices() []device.D {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	devices := make([]device.D, 0, len(reg.devices))
	for _, e := range reg.devices {
		if !device.IsDone(e.device) {
			devices = append(devices, e.device)
		}
	}
	return devices
}

// Observe observes the supplied discovery headers. This will add the device if
// it has not been observed before, or refresh its timeout and metadata if it
// has.
func (reg *Registry) Observe(dh *protocol.DiscoveryHeaders) (d device.D, isNew bool) {
	d, isNew = reg.observeImpl(dh)

	if reg.DeviceRegistry != nil {
		reg.DeviceRegistry.Add(d)
	}

	return
}

func (reg *Registry) observeImpl(dh *protocol.DiscoveryHeaders) (d device.D, isNew bool) {
	// Use hardware address as ID.
	id := dh.HardwareAddr().String()
	now := time.Now()

	reg.mu.Lock()
	defer reg.mu.Unlock()

	// Unregister any entries that are currently done, under lock. This prevents
	// a race where the device is Done, and will be unregistered, but it is then
	// rediscovered, causing the registration to fail as duplicate and be missed.
	reg.unregisterDoneEntriesLocked()

	// Do we already have an entry for this device?
	e := reg.devices[id]
	if e == nil {
		// Create a remote device.
		d := device.MakeRemote(id, dh)

		// This is a new entry.
		e = &registryEntry{
			reg:               reg,
			device:            d,
			deviceID:          id,
			updateExpirationC: make(chan time.Time, 1),
		}

		// Unregister this entry when it expires.
		go e.manageEntryLifecycle()

		if reg.devices == nil {
			reg.devices = make(map[string]*registryEntry)
		}
		reg.devices[id] = e
		isNew = true
	}

	// Observe the entry and update its timeout and headers.
	st := discoveredDeviceState{
		dh: dh,
	}
	if reg.Expiration > 0 {
		st.expires = now.Add(reg.Expiration)
	}
	e.updateState(now, &st)

	d = e.device
	return
}

// Unregister unregisters and shuts down the specified device.
//
// If the device is not currently registered, Unregister will do nothing.
func (reg *Registry) Unregister(d device.D) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	e := reg.devices[d.ID()]
	if e != nil {
		// Explicitly mark the device as done and unregister it.
		//
		// This follows the same path done in manageEntryLifecycle's defer
		// statements when a device naturally expires.
		e.device.MarkDone()
		reg.unregisterEntryLocked(e)
	}
}

func (reg *Registry) unregisterDoneEntriesLocked() {
	for _, d := range reg.devices {
		if device.IsDone(d.device) {
			reg.unregisterEntryLocked(d)
		}
	}
}

func (reg *Registry) unregisterEntry(e *registryEntry) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.unregisterEntryLocked(e)
}

func (reg *Registry) unregisterEntryLocked(e *registryEntry) {
	if re := reg.devices[e.deviceID]; re != e {
		// This device is Already unregistered. This can happen if the entry
		// self-unregisters while it's shutting itself down, but it's already been
		// explicitly deleted.
		return
	}

	// Entry can no longer receive updates.
	close(e.updateExpirationC)

	// Remove this entry from the devices map.
	delete(reg.devices, e.deviceID)
}

type registryEntry struct {
	// reg is the parent registry.
	reg *Registry

	// device is the discovered device entry.
	device *device.Remote
	// deviceID is a copy of device's ID.
	deviceID string

	// updateExpirationC is an internal channel used to send new expiration times
	// to the device.
	updateExpirationC chan time.Time
}

func (e *registryEntry) updateState(now time.Time, st *discoveredDeviceState) {
	e.device.UpdateHeaders(now, st.dh)
	if !st.expires.IsZero() {
		e.updateExpirationC <- st.expires
	}
}

func (e *registryEntry) manageEntryLifecycle() {
	// When we finish lifecycle management, unregister the entry and mark it
	// done.
	defer func() {
		e.device.MarkDone()
		e.reg.unregisterEntry(e)
	}()

	var t *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-e.device.DoneC():
			// The device has closed.
			return

		case <-timerC:
			// This entry has expired.
			return

		case expireTime, ok := <-e.updateExpirationC:
			if !ok {
				// This entry has been closed.
				return
			}

			// Calculate the expiration delta.
			expirationDelta := expireTime.Sub(time.Now())
			if expirationDelta < 0 {
				// We are already expired!
				return
			}

			// Initialize or reset our expiration timer.
			if t == nil {
				// First run, initialize the timer.
				t = time.NewTimer(expirationDelta)
				defer t.Stop()
			} else {
				// Reset the timer for the next expiration.
				if !t.Stop() {
					<-t.C
				}
				t.Reset(expirationDelta)
			}
			timerC = t.C
		}
	}
}
