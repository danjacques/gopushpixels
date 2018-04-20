// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package proxy

import (
	"net"
	"sort"
	"sync"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/bufferpool"
	"github.com/danjacques/gopushpixels/support/logging"
	"github.com/danjacques/gopushpixels/support/network"
)

// Manager manages the current proxy state.
//
// For every Device registered with the Manager, a proxy Device is created and
// associated with that device.
type Manager struct {
	// ProxyAddr is the local address that the proxy devices should listen on.
	//
	// If nil, the default UDP address will be chosen. This is probably not what
	// you want, since the device has to choose an IP address to identify as in
	// its discovery broadcast, and the default address is usually 0.0.0.0.
	ProxyAddr net.IP

	// AddressRegistry is a registry used to assign hardware addresses
	// (device IDs) to proxy devices.
	AddressRegistry AddressRegistry

	// GroupOffset is an offset that is added to the group identifier of
	// proxied devices. This can be used to differentiate proxy devices from
	// their base ones.
	//
	// For example, with proxy offset 16, a device with group 2 would have its
	// proxy device broadcast as group 18 (2+16).
	GroupOffset int32

	// Logger is the logger instance to use. If nil, no logs will be generated.
	Logger logging.L

	mu      sync.RWMutex
	devices map[string]*Device

	// proxyLeases is a map of current proxy lease holders. If this map contains
	// any entries, then the proxy will refrain from forwarding packets, assuming
	// that something else is using those devices.
	proxyLeases map[interface{}]struct{}

	// listeners is the set of registered listeners.
	//
	// It is managed independently from the fields protected by mu.
	listeners sync.Map

	// udpPacketPool is a pool of buffers to use and reuse for proxy device
	// UDP packet data.
	//
	// It will be initialized when the first proxy device is added.
	udpPacketPool *bufferpool.Pool
}

// ProxyDevices returns the list of registered proxy devices. They will be
// sorted by ID.
func (m *Manager) ProxyDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.devices) == 0 {
		return nil
	}
	devices := make([]*Device, 0, len(m.devices))
	for _, pd := range m.devices {
		devices = append(devices, pd)
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID() < devices[j].ID()
	})
	return devices
}

// AddListener adds a Listener to this Manager.
func (m *Manager) AddListener(l Listener) { m.listeners.Store(l, nil) }

// RemoveListener removes a Listener from the Manager.
func (m *Manager) RemoveListener(l Listener) { m.listeners.Delete(l) }

// AddDevice adds a new device to the proxy manager.
//
// The device will remain active in the proxy until it expires or its closed,
// at which point it will be automatically removed.
func (m *Manager) AddDevice(d device.D) error {
	baseID := d.ID()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Make sure our packet pool is initialized.
	if m.udpPacketPool == nil {
		m.udpPacketPool = &bufferpool.Pool{
			Size: network.MaxUDPSize,
		}
	}

	// Do we already have an entry for this device? If so, ignore it.
	if _, ok := m.devices[baseID]; ok {
		m.logger().Debugf("Device %q is already proxied; ignoring.", baseID)
		return nil
	}

	m.logger().Debugf("Registering managed device %s...", baseID)

	// Generate a hardware address for this device. We must do this
	// deterministically, so our proxy devices have the same addresses as their
	// proxied devices. To do this, we'll use a hash.
	addr, err := m.AddressRegistry.Generate(baseID)
	if err != nil {
		return err
	}

	// Create a proxy device for this device.
	pd, err := makeProxyDevice(m, d, addr)
	if err != nil {
		return err
	}

	// Create and register a managed entry for this device.
	if m.devices == nil {
		m.devices = make(map[string]*Device)
	}
	m.devices[baseID] = pd

	m.logger().Infof("Created proxy device %q on %q for device %s.", pd.ID(), pd.Addr(), baseID)
	return nil
}

// Close terminates all proxies and shuts down the manager.
func (m *Manager) Close() error {
	// Take out our own lease, stopping forwarding permanently.
	m.AddLease(m)

	// Get a list of current proxy devices to close.
	devices := m.ProxyDevices()

	// Shutdown each device and Wait for it to complete.
	//
	// NOTE: We can't hold lock for this, since devices use it when unregistering.
	for _, pd := range devices {
		// Shutdown the device and wait for it to complete.
		pd.shutdown()
	}

	return nil
}

// IsProxyDeviceAddr is a convenience method for calling m's AddressRegistry's
// function of the same name.
func (m *Manager) IsProxyDeviceAddr(addr net.HardwareAddr) bool {
	return m.AddressRegistry.IsProxyDeviceAddr(addr)
}

// AddLease adds l as a proxy traffic blocking lessee.
func (m *Manager) AddLease(l interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addLeaseLocked(l)
}

// RemoveLease removes l as a proxy traffic blocking lessee.
func (m *Manager) RemoveLease(l interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.proxyLeases[l]; ok {
		delete(m.proxyLeases, l)
		proxyLeaseGauge.Dec()
	}
}

func (m *Manager) addLeaseLocked(l interface{}) {
	if m.proxyLeases == nil {
		m.proxyLeases = make(map[interface{}]struct{})
	}

	if _, ok := m.proxyLeases[l]; !ok {
		m.proxyLeases[l] = struct{}{}
		proxyLeaseGauge.Inc()
	}
}

// Forwarding returns whether or not this ProxyManager is configured to forward
// packets to its proxies.
func (m *Manager) Forwarding() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.proxyLeases) == 0
}

// removeDevice is called by a Device's shutdown method to unregister it from
// its Manager.
//
// If you are considering adding another user of this method, make sure to
// consider locking with respect to the proxy device's self-initiated
// unregistration.
func (m *Manager) removeDevice(pd *Device) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.devices, pd.baseID)

	// Update our device monitoring.
	pd.monitoring.Update(pd)
}

func (m *Manager) hasListeners() (has bool) {
	// Check for listeners by performing a Range over them. If we have any
	// elements, then we have listeners.
	m.listeners.Range(func(_, _ interface{}) bool {
		has = true
		return false
	})
	return
}

func (m *Manager) sendPacketToListeners(d device.D, pkt *protocol.Packet, forwarded bool) {
	m.listeners.Range(func(l, _ interface{}) bool {
		// Send the packet to our listeners in the list. Doing this without being
		// locked allows the listeners to unregister themselves even as packets are
		// being delivered.
		l.(Listener).ReceivePacket(d, pkt, forwarded)
		return true
	})
}

func (m *Manager) logger() logging.L { return logging.Must(m.Logger) }
