// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/network"
)

// Sender is an interface that can dispatch data and packets to a single
// device.
//
// Sender is not safe for concurrent use.
type Sender interface {
	// DatagramSender sends a raw datagram to the underlying device.
	//
	// Generally, users should prefer to send packets via SendPacket over
	// SendDatagram.
	network.DatagramSender

	// SendPacket writes the contents of packet to the target device.
	//
	// Unlike SendDatagram, SendPacket has the opportunity to examine the content
	// and intent of the packet and determine how to optimally send it to the
	// target device.
	//
	// Regardless of any internal buffering, SendPacket will not retain any of
	// packet.
	SendPacket(packet *protocol.Packet) error
}
