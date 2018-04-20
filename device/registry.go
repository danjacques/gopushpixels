// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"sort"
	"sync"
)

// Registry is a generic device registry. It tracks devices by ID, records
// which group devices belong to, and removes device entries when they expire.
type Registry struct {
	mu sync.RWMutex
	// Map of active devices.
	devices map[string]*registryEntry
	// Maintain a map of group IDs to the devices that are in them.
	groupMap map[int]map[*registryEntry]struct{}
	// Maintain a map of the devices that claim an Ordinal.
	ordinalMap map[Ordinal]map[*registryEntry]struct{}
}

// Add adds or update's d's registration in the Registry.
func (reg *Registry) Add(d D) {
	id := d.ID()

	// Fast path (read lock): Device is registered, nothing's changed.
	if reg.checkDeviceRegistration(id) {
		return
	}

	reg.mu.Lock()
	defer reg.mu.Unlock()

	// Unregister any entries that are currently done, under lock. This prevents
	// a race where the device is Done, and will be unregistered, but it is then
	// rediscovered, causing the registration to fail as duplicate and be missed.
	reg.unregisterDoneEntriesLocked()

	isNew := false
	e := reg.devices[id]
	if e == nil {
		// This is a new entry.
		e = &registryEntry{
			reg:      reg,
			device:   d,
			deviceID: id,
		}

		if reg.devices == nil {
			reg.devices = make(map[string]*registryEntry)
		}
		reg.devices[id] = e
		isNew = true
	}

	// Update our device group accounting.
	reg.updateOrdinalLocked(e, isNew)

	if isNew {
		// Unregister the device from the Registry when it is Done.
		go e.manageEntryLifecycle()
	}
}

// checkDeviceRegistration checks that the device for id is completely
// registered and up-to-date under read lock. This is faster than taking a
// write lock, and will generally be true for all devices.
func (reg *Registry) checkDeviceRegistration(id string) bool {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	// Is the device registered?
	e := reg.devices[id]
	if e == nil {
		return false
	}

	// Is the device properly registered for its group and ordinal?
	ord := e.device.Ordinal()
	if _, ok := reg.groupMap[ord.Group][e]; !ok {
		return false
	}
	if _, ok := reg.ordinalMap[ord][e]; !ok {
		return false
	}

	// Finally, is the device Done? If so, we will have to reregister.
	if IsDone(e.device) {
		return false
	}
	return true
}

// Get returns the registered device for the specified ID.
//
// If no device is registered for this ID, Get will return nil.
func (reg *Registry) Get(id string) D {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	e := reg.devices[id]
	if e == nil {
		return nil
	}

	// Omit this device if it's Done.
	if IsDone(e.device) {
		return nil
	}
	return e.device
}

// GetUniqueOrdinal returns the registered device for the specified ordinal.
//
// If there is no device that is uniquely registered for o, GetOrdinal will
// return nil.
func (reg *Registry) GetUniqueOrdinal(o Ordinal) D {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	emap := reg.ordinalMap[o]
	if len(emap) == 1 {
		// Exactly one device is registered for this Ordinal.
		for e := range emap {
			// Omit this device if it's Done.
			if IsDone(e.device) {
				return nil
			}
			return e.device
		}
	}
	return nil
}

// AllGroups returns all registered groups and their respective devices.
func (reg *Registry) AllGroups() map[int][]D {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	if len(reg.groupMap) == 0 {
		return nil
	}
	groups := make(map[int][]D, len(reg.groupMap))
	for group := range reg.groupMap {
		devices := reg.devicesForGroupLocked(group)
		if len(devices) > 0 {
			groups[group] = devices
		}
	}
	return groups
}

// DevicesForGroup returns the devices for the specified group.
//
// If no devices are registered in that group, it will return an empty slice.
func (reg *Registry) DevicesForGroup(group int) []D {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return reg.devicesForGroupLocked(group)
}

func (reg *Registry) devicesForGroupLocked(group int) []D {
	devices := reg.groupMap[group]
	if len(devices) == 0 {
		return nil
	}

	result := make([]D, 0, len(devices))
	for e := range devices {
		// Omit this device if it's Done.
		if IsDone(e.device) {
			continue
		}

		result = append(result, e.device)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID() < result[j].ID()
	})
	return result
}

func (reg *Registry) updateOrdinalLocked(e *registryEntry, isNew bool) {
	ordinal := e.device.Ordinal()

	// Registration in group map.
	if isNew || e.registeredGroup != ordinal.Group {
		// The device has never been registered, or has switched groups. Update.
		if !isNew {
			reg.removeFromGroupMapLocked(e)
		}

		if reg.groupMap == nil {
			reg.groupMap = make(map[int]map[*registryEntry]struct{})
		}
		entryMap := reg.groupMap[ordinal.Group]
		if entryMap == nil {
			entryMap = make(map[*registryEntry]struct{})
			reg.groupMap[ordinal.Group] = entryMap
		}
		entryMap[e] = struct{}{}
		e.registeredGroup = ordinal.Group
	}

	// Registration in ordinal map.
	if isNew || e.registeredOrdinal != ordinal {
		// The device has never been registered, or has switched ordinals. Update.
		if !isNew {
			reg.removeFromOrdinalMapLocked(e)
		}

		if reg.ordinalMap == nil {
			reg.ordinalMap = make(map[Ordinal]map[*registryEntry]struct{})
		}
		entryMap := reg.ordinalMap[ordinal]
		if entryMap == nil {
			entryMap = make(map[*registryEntry]struct{})
			reg.ordinalMap[ordinal] = entryMap
		}
		entryMap[e] = struct{}{}
		e.registeredOrdinal = ordinal
	}
}

func (reg *Registry) removeFromGroupMapLocked(e *registryEntry) {
	delete(reg.groupMap[e.registeredGroup], e)
	if len(reg.groupMap[e.registeredGroup]) == 0 {
		delete(reg.groupMap, e.registeredGroup)
	}
	e.registeredGroup = 0
}

func (reg *Registry) removeFromOrdinalMapLocked(e *registryEntry) {
	delete(reg.ordinalMap[e.registeredOrdinal], e)
	if len(reg.ordinalMap[e.registeredOrdinal]) == 0 {
		delete(reg.ordinalMap, e.registeredOrdinal)
	}
	e.registeredOrdinal = Ordinal{}
}

func (reg *Registry) unregisterDoneEntriesLocked() {
	for _, e := range reg.devices {
		if IsDone(e.device) {
			reg.unregisterEntryLocked(e)
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
		// This device is already unregistered. This can happen if the entry
		// self-unregisters while it's shutting itself down, but it's already been
		// explicitly deleted.
		return
	}

	// Remove this entry from its group/ordinal maps.
	reg.removeFromGroupMapLocked(e)
	reg.removeFromOrdinalMapLocked(e)

	// Remove this entry from the devices map.
	delete(reg.devices, e.deviceID)
}

type registryEntry struct {
	// reg is the parent registry.
	reg *Registry

	// device is the discovered device entry.
	device D
	// deviceID is a copy of device's ID.
	deviceID string

	registeredGroup   int
	registeredOrdinal Ordinal
}

func (e *registryEntry) manageEntryLifecycle() {
	<-e.device.DoneC()
	e.reg.unregisterEntry(e)
}
