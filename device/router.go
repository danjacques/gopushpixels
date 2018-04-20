// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"sync"

	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/logging"

	"github.com/pkg/errors"
)

// ErrNoRoute is a sentinel error that is returned by a Router's Route command
// when the requested device is not registered.
var ErrNoRoute = errors.New("no route for device")

// Listener is registered with a Router, and receives a callback for each routed
// packet.
type Listener interface {
	// HandlePacket is called for each Packet sent to this Router.
	//
	// pkt is a shared read-only object, and must not be modified.
	HandlePacket(d D, pkt *protocol.Packet)
}

// ListenerFunc generates a Listsner whose HandlePacket method calls the
// supplied function.
func ListenerFunc(fn func(D, *protocol.Packet)) Listener {
	return &listenerFunc{fn}
}

type listenerFunc struct {
	fn func(D, *protocol.Packet)
}

func (lf *listenerFunc) HandlePacket(d D, pkt *protocol.Packet) { lf.fn(d, pkt) }

// Router contains a registry of devices. Once registered, a device remains with
// the Router until it is either removed or marks itself done via DoneC.
//
// A Router accepts a series of packets delivered to a specific device ID, and
// sends those packets to that device via a PacketWriter.
//
// A Router operates on a device ID's Value field, rather than the full ID,
// since its targeted at sending recorded data, which will not include the ID's
// Type parameter.
type Router struct {
	// Registry is the device registry. It is used to identify devices that will
	// receive routed packets.
	//
	// Registry must not be nil
	Registry *Registry
	// Logger is the logger that this Router should use.
	//
	// Setting or changing Logger should be done during Router setup, and is
	// not safe for concurrent use.
	Logger logging.L

	// listeners is a list of registered listeners.
	listeners sync.Map

	mu sync.RWMutex
	// connections is the set of open router connections.
	connections map[D]*routerConnection
}

// Route sends a packet to the device identified by the specified ordinal or id.
//
// If the ordinal is valid and uniquely registered, the device registered to
// that ordinal will receive the packet. Otherwise, if the device ID is
// registered, it will receive the packet.
func (r *Router) Route(ordinal Ordinal, id string, pkt *protocol.Packet) error {
	// First, see if we can find a device registered to the specified ordinal.
	var d D
	if ordinal.IsValid() {
		d = r.Registry.GetUniqueOrdinal(ordinal)
	}
	if d == nil {
		// No device is uniquely registered to this ordinal; try the ID.
		d = r.Registry.Get(id)
	}
	if d == nil {
		// No registry entry for this device.
		return ErrNoRoute
	}

	// Get or create a connection to d.
	rc, err := r.getOrCreateConnection(d)
	if err != nil {
		return err
	}

	// Dispatch the packet to all listeners.
	r.dispatchPacketToListeners(rc.device, pkt)

	// Send the packet immediately. Our packet dispatch goroutines can send it
	// while our listeners are procesing it.
	return rc.sendPacket(pkt)
}

// AddListener registers a Listener with this Router.
func (r *Router) AddListener(l Listener) { r.listeners.Store(l, nil) }

// RemoveListener removes a Listener from this Router.
//
// If l is not registered, nothing will happen.
func (r *Router) RemoveListener(l Listener) { r.listeners.Delete(l) }

func (r *Router) getOrCreateConnection(d D) (*routerConnection, error) {
	// Fast path: is a connection already registered for this device?
	r.mu.RLock()
	rc := r.connections[d]
	r.mu.RUnlock()

	// If the connection isn't done, return it.
	if rc != nil && !rc.isDone() {
		return rc, nil
	}

	// Slow path: create a new connection for this device.
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear any Done registrations before processing. This ensures that if a
	// device flickers on and off, we don't encounter a race where its channel
	// closes but we haven't unregistered it yet, so we still see it as a
	// duplicate.
	r.clearDoneLocked()

	// A connection may have been created before we acquired an exclusive lock;
	// check again.
	if rc = r.connections[d]; rc != nil {
		return rc, nil
	}

	// Create a Sender for this device.
	s, err := d.Sender()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Sender")
	}

	const bufferSize = 1024
	rc = &routerConnection{
		r:      r,
		device: d,
		sender: s,
	}

	if r.connections == nil {
		r.connections = make(map[D]*routerConnection)
	}
	r.connections[d] = rc

	// Monitor this connection and close it when it's device has finished.
	go rc.manageConnectionLifecycle()

	return rc, nil
}

func (r *Router) dispatchPacketToListeners(d D, pkt *protocol.Packet) {
	r.listeners.Range(func(l, _ interface{}) bool {
		l.(Listener).HandlePacket(d, pkt)
		return true
	})
}

// Shutdown all routes and resources used by the Router.
func (r *Router) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Unregister all entries.
	for _, rc := range r.connections {
		r.unregisterConnectionLocked(rc)
	}
}

func (r *Router) unregisterEntry(rc *routerConnection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unregisterConnectionLocked(rc)
}

func (r *Router) unregisterConnectionLocked(rc *routerConnection) {
	delete(r.connections, rc.device)
	rc.shutdown()
}

func (r *Router) clearDoneLocked() {
	for _, rc := range r.connections {
		if rc.isDone() {
			r.unregisterConnectionLocked(rc)
		}
	}
}

func (r *Router) logger() logging.L { return logging.Must(r.Logger) }

type routerConnection struct {
	// r is the Router that owns routerConnection.
	r *Router

	// device is the device that this entry represents.
	device D

	// sender is a device-specific packet Sender.
	sender       Sender
	shutdownOnce sync.Once
}

func (rc *routerConnection) sendPacket(pkt *protocol.Packet) error {
	return rc.sender.SendPacket(pkt)
}

func (rc *routerConnection) isDone() bool {
	select {
	case <-rc.device.DoneC():
		return true
	default:
		return false
	}
}

func (rc *routerConnection) shutdown() {
	rc.shutdownOnce.Do(func() {
		// Shut down our writer.
		_ = rc.sender.Close()
	})
}

func (rc *routerConnection) manageConnectionLifecycle() {
	// Block until the underlying device is done.
	<-rc.device.DoneC()
	rc.shutdown()
}
