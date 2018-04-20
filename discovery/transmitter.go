// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package discovery

import (
	"bytes"

	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/fmtutil"
	"github.com/danjacques/gopushpixels/support/logging"
	"github.com/danjacques/gopushpixels/support/network"
)

// DefaultTransmitterConn returns a resolved connection configuration bound to
// the default device discovery port and all-hosts multicast broadcast address.
func DefaultTransmitterConn() *network.ResolvedConn {
	return network.UDP4MulticastTransmitterConn(protocol.DiscoveryUDPPort)
}

// Transmitter broadcasts discovery data for a set of devices.
//
// It can be run periodically to broadcast the existence of several devices.
//
// Transmitter is not safe for concurrent use.
type Transmitter struct {
	// Logger, if not nil, is the Logger to log Listener status to.
	Logger logging.L

	buf bytes.Buffer
}

// Broadcast broadcasts discovery headers.
func (t *Transmitter) Broadcast(w network.DatagramSender, dh *protocol.DiscoveryHeaders) error {
	// Clear our buffer from any previous instance.
	t.buf.Grow(network.MaxUDPSize)
	t.buf.Reset()

	// Build the discovery header.
	if err := dh.WritePacket(&t.buf); err != nil {
		return err
	}

	// Send the discovery packet.
	t.logger().Debugf("Broadcasting device %q:\n%s", dh.HardwareAddr(), fmtutil.Hex(t.buf.Bytes()))
	return w.SendDatagram(t.buf.Bytes())
}

func (t *Transmitter) logger() logging.L { return logging.Must(t.Logger) }
