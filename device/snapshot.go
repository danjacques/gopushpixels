// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"sync"
	"time"

	"github.com/danjacques/gopushpixels/pixel"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/protocol/pixelpusher"
)

// Snapshot represents a snapshot of the device state.
//
// A Snapshot is an independent data clone.
//
// Currently this only supports PixelPusher device states.
type Snapshot struct {
	// ID is the snapshot device ID.
	ID string

	// Strips is the set of strips on this device.
	Strips []*pixelpusher.StripState
}

// SnapshotManager manages device state snapshots.
//
// TODO: Generalize for more than PixelPusher.
//
// SnapshotManager is safe for concurrent use.
type SnapshotManager struct {
	// SampleRate is the snapshot sample rate. After a sample is taken, any
	// further samples will be ignored until SampleRate has passed.
	//
	// If SampleRate is <= 0, all samples will be taken.
	SampleRate time.Duration

	mu     sync.RWMutex
	states map[string]*snapshotDeviceState

	// lastSnapshotTimeMu protects lastSnapshotTime.
	lastSnapshotTimeMu sync.RWMutex
	// lastSnapshotTime is the last minimum snapshot time that was calculated.
	// It is protected by mu.
	lastSnapshotTime time.Time
}

// HandlePacket accepts a packet, pkt, and updates its associated strip snapshot
// based on the packet's contents. If the packet isn't a pixel packet, it will
// be ignored.
func (m *SnapshotManager) HandlePacket(d D, pkt *protocol.Packet) {
	// Get our device state for d.
	ds := m.getOrCreateDeviceState(d)

	switch {
	case pkt.PixelPusher != nil:
		pp := pkt.PixelPusher
		if len(pp.StripStates) > 0 {
			ds.updatePixelPusherStrips(pp.StripStates)
		}
	}
}

func (m *SnapshotManager) getDeviceState(id string) *snapshotDeviceState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[id]
}

func (m *SnapshotManager) getOrCreateDeviceState(d D) *snapshotDeviceState {
	id := d.ID()

	// See if the device is already registered under read-lock. This is likely.
	ds := m.getDeviceState(id)
	if ds != nil {
		return ds
	}

	// The device doesn't exist; create it.
	m.mu.Lock()
	defer m.mu.Unlock()

	ds = m.states[id]
	if ds != nil {
		// The device was created since we checked under read-lock. Return.
		return ds
	}

	ds = &snapshotDeviceState{
		m:      m,
		id:     id,
		device: &Mutable{},
	}
	ds.device.Initialize(d.DiscoveryHeaders())

	if m.states == nil {
		m.states = make(map[string]*snapshotDeviceState)
	}
	m.states[id] = ds

	// Unregister this from the SnapshotManager when the device closes. This
	// avoids indefinite accumultaion of snapshot state as devices come and go.
	go func() {
		<-d.DoneC()
		m.Delete(d)
	}()

	return ds
}

// SnapshotForDevice returns the current snapshot for the specified device.
func (m *SnapshotManager) SnapshotForDevice(d D) *Snapshot {
	ds := m.getDeviceState(d.ID())
	if ds == nil {
		return nil
	}
	return ds.getSnapshot()
}

// HasSnapshotForDevice returns true if a snapshot is stored for d.
func (m *SnapshotManager) HasSnapshotForDevice(d D) bool {
	ds := m.getDeviceState(d.ID())
	if ds == nil {
		return false
	}
	return ds.hasSnapshot()
}

// Delete removes any stored snapshot state for d.
func (m *SnapshotManager) Delete(d D) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, d.ID())
}

// shouldSnapshot returns true if the device, which last took a snapshot
// at last, should take a snapshot.
//
// If no SampleRate is set, shouldSnapshot will always return true.
//
// If a sample rate is set, shouldSnapshot will examine lastSnapshotTime. If
// last is before that time, shouldSnapshot will return true. Additionally, if
// now is at least one SampleRate past that time, the threshold will be updated.
//
// This method attempts to coordinate snapshots such that they occur around the
// along a common edge across all devices and strips.
//
// This handles the following cases:
//
//	              SampleRate    SampleRate
//	   |-------LE------------NE------------NE2
//	           |             |              |
//	1)   last  |     now     |              |
//	2)         |  last   now |              |
//	3)      last...          |   now        |
//	4)      last...          |              |  now
func (m *SnapshotManager) shouldSnapshot(now, last time.Time) bool {
	if m.SampleRate <= 0 {
		return true
	}

	// (Fast path) check the time under read lock.
	m.lastSnapshotTimeMu.RLock()
	lastEdge := m.lastSnapshotTime
	m.lastSnapshotTimeMu.RUnlock()

	nextEdge := lastEdge.Add(m.SampleRate)
	if !nextEdge.Before(now) {
		// The next sample edge is in the future. Return true only if last is
		// before the current edge.
		//
		// (1) if last is < lastEdge (true), (2) if last is >= lastEdge (false).
		return last.Before(lastEdge)
	}

	// The next sample edge is now, or in the past. Update the sample edge under
	// write lock.
	//
	// If we are within one sample edge of nextEdge, then update to nextEdge. This
	// ensures a constant update window. Otherwise, reset our timing to the
	// current time.
	//
	// It's possible that someone else has already updated the edge, so only
	// update if we are still out of date after acquiring the lock.
	//
	// At this point, since we're moving the edge, "last" must be before the new
	// edge, so we'll always return true.
	m.lastSnapshotTimeMu.Lock()
	nextEdge = m.lastSnapshotTime.Add(m.SampleRate)
	if now.After(nextEdge) {
		lastEdge, nextEdge = nextEdge, nextEdge.Add(m.SampleRate)
		if now.Before(nextEdge) {
			// (3)
			m.lastSnapshotTime = lastEdge
		} else {
			// (4)
			m.lastSnapshotTime = now
		}
	}
	m.lastSnapshotTimeMu.Unlock()

	return true
}

// snapshotDeviceState is a snapshot state for a single device.
type snapshotDeviceState struct {
	m  *SnapshotManager
	id string

	mu             sync.RWMutex
	device         *Mutable
	lastSampleTime map[pixelpusher.StripNumber]time.Time
}

func (ds *snapshotDeviceState) updatePixelPusherStrips(strips []*pixelpusher.StripState) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	now := time.Now()
	for _, ss := range strips {
		ds.updatePixelPusherStripLocked(now, ss.StripNumber, &ss.Pixels)
	}
}

func (ds *snapshotDeviceState) updatePixelPusherStripLocked(now time.Time,
	stripNumber pixelpusher.StripNumber, pixels *pixel.Buffer) {

	// If we are sampling, are we within our sample window?
	sr := ds.m.SampleRate
	if sr > 0 {
		// Get the last-sampled time for this strip.
		lastSampleTime := ds.lastSampleTime[stripNumber]
		if !ds.m.shouldSnapshot(now, lastSampleTime) {
			// Decided not to snapshot.
			return
		}
	}

	// Update our pixel record.
	ds.device.SetPixels(int(stripNumber), pixels)

	// Record this moment as the last sample time.
	if sr > 0 {
		if ds.lastSampleTime == nil {
			ds.lastSampleTime = make(map[pixelpusher.StripNumber]time.Time, ds.device.NumStrips())
		}
		ds.lastSampleTime[stripNumber] = now
	}
}

func (ds *snapshotDeviceState) hasSnapshot() bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.device.NumStrips() > 0
}

func (ds *snapshotDeviceState) getSnapshot() *Snapshot {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	ss := Snapshot{
		ID:     ds.id,
		Strips: make([]*pixelpusher.StripState, ds.device.NumStrips()),
	}
	for i := range ss.Strips {
		var clone pixelpusher.StripState
		clone.StripNumber = pixelpusher.StripNumber(i)
		ds.device.ClonePixelsTo(i, &clone.Pixels)
		ss.Strips[i] = &clone
	}

	return &ss
}
