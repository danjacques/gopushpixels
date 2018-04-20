// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

// Package device provides basic device definition and management.
//
// This package offers a definition of a basic device, D, as well as several
// implementations of D.
//
// Users wishing to interact with real devices will find better utility in the
// "discovery" package, which uses this package to instantiate Remote devices
// for each discovered device.
//
// Mutable is offered to track the pixel state of a given device and generate
// mutation packets to sync the device to that state.
//
// Snapshot can be used to store a (potentially-sampled) pixel state of a
// given of device.
//
// Optional Prometheus monitoring can be enabled by registering on startup
// (generally init()) via RegisterMonitoring.
package device

import (
	"net"
	"time"

	"github.com/danjacques/gopushpixels/protocol"
)

// Info is a set of stats collected for this device.
type Info struct {
	PacketsReceived int64
	BytesReceived   int64

	PacketsSent int64
	BytesSent   int64

	Created  time.Time
	Observed time.Time
}

// D is a single device. It implements a generic device interface.
type D interface {
	// ID is this device's ID. It should be unique (within this system) to this
	// device, and should be consistent between executions, regardless of simple
	// reconfiguations on the device's part.
	//
	// A hardware address is suitable for this purpose.
	ID() string

	// Ordinal returns this Device's ordinal value.
	Ordinal() Ordinal

	// Sender returns a device Sender instance.
	//
	// Multiple Senders can be created for the same device; however, any
	// individual Sender is not safe for concurrent use (see Sender interface
	// for more information).
	//
	// It is the caller's responsibility to close the Sender when finished.
	Sender() (Sender, error)

	// DiscoveryHeaders returns the device's discovery protocol headers.
	//
	// DiscoveryHeaders may return nil if no headers are available.
	//
	// DiscoveryHeaders can be used to obtain a protocol.PacketReader, if packet
	// parsing is required.
	DiscoveryHeaders() *protocol.DiscoveryHeaders

	// DoneC is closed when this D is no longer considered active.
	DoneC() <-chan struct{}

	// Addr is this device's address. It may be nil.
	Addr() net.Addr

	// Info returns the current information and stats for this device.
	Info() Info
}

// IsDone returns true if this device is done (its DoneC is closed).
func IsDone(d D) bool {
	select {
	case <-d.DoneC():
		return true
	default:
		return false
	}
}
