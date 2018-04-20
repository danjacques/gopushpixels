// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package proxy

import (
	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"
)

// Listener defines a packet listener. A Listener can register with a Manager
// to receive proxy-targeted packets as they arrive.
type Listener interface {
	ReceivePacket(d device.D, pkt *protocol.Packet, forwarded bool)
}

// ListenerFunc is a function that conforms to a Listener interface.
type funcListener struct {
	fn func(device.D, *protocol.Packet, bool)
}

// ListenerFunc returns a Listener bound to a function.
func ListenerFunc(fn func(device.D, *protocol.Packet, bool)) Listener {
	return &funcListener{fn}
}

func (fl *funcListener) ReceivePacket(d device.D, pkt *protocol.Packet, forwarded bool) {
	fl.fn(d, pkt, forwarded)
}
