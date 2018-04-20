// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"fmt"
	"net"
	"sync"

	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/bufferpool"
	"github.com/danjacques/gopushpixels/support/fmtutil"
	"github.com/danjacques/gopushpixels/support/logging"
	"github.com/danjacques/gopushpixels/support/network"

	"github.com/pkg/errors"
)

// Local is a local "virtual' device. A Local allows your local system to
// instantiate its own devices. Local can be useful for testing and the
// simluation of devices.
//
// Local's exported fields must not be changed after Start is called.
type Local struct {
	// The local device's ID.
	DeviceID string

	// OnPacketData is wthe callback that is called when new packet data is
	// received.
	//
	// OnPacketData must not be nil.
	//
	// The packet data is owned by a bufferpool.Pool. Recipients of the buffer may
	// Retain it and Release it to prevent it from reentering the pool. The buffer
	// that is handed to the callback is automatically Released when the callback
	// returns; the callback SHOULD NOT release the buffer.
	OnPacketData func(buf *bufferpool.Buffer)

	// UDPPacketPool, if not nil, is the packet pool to use for UDP packet data.
	//
	// If nil, a local packet pool will be generated and used.
	UDPPacketPool *bufferpool.Pool

	// Logger, if not nil, is the logger to use to log events.
	//
	// Changes to Logger after Start is called will have no effect.
	Logger logging.L

	// logger is the resolved logger on Start.
	logger logging.L

	// addr is the address of the remote device.
	addr *net.UDPAddr
	conn *net.UDPConn

	// doneC is used to implement DoneC().
	doneC chan struct{}

	// monitoring is the device's monitoring state.
	monitoring Monitoring

	// udpPacketPool is a pool of buffers to use and reuse for UDP packet data.
	packetPool *bufferpool.Pool

	// listenDoneC is used to signal that our listen goroutine has finished.
	listenDoneC chan struct{}

	// mu protects the following data.
	mu sync.RWMutex
	// dh is the set of retained DiscoveryHeaders.
	dh *protocol.DiscoveryHeaders
}

var _ D = (*Local)(nil)

// Start finishes device setup and begins listening for packets.
//
// The returned device presumes ownership over conn, and will close it when
// closed.
func (d *Local) Start(conn *net.UDPConn) {
	switch {
	case d.conn != nil:
		panic("already started")
	case d.OnPacketData == nil:
		panic("no OnPacketData callback defined")
	}

	// Resolve our logger.
	d.logger = logging.Must(d.Logger)

	d.conn = conn
	d.addr = conn.LocalAddr().(*net.UDPAddr)
	d.doneC = make(chan struct{})

	// Configure our packet buffer pool.
	d.packetPool = d.UDPPacketPool
	if d.packetPool == nil {
		d.packetPool = &bufferpool.Pool{
			Size: network.MaxUDPSize,
		}
	}

	// Listen for packets to this Local in a separate goroutine.
	d.listenDoneC = make(chan struct{})
	go d.listenForPackets()

	// Update monitoring information.
	d.monitoring.Update(d)
}

// Close closes the Local, freeing its remote connection resource and marking it
// Done.
//
// After Close has returned, no more packet callbacks will be sent.
func (d *Local) Close() error {
	// Clouse our done channel. This notifies our callers that we have finished,
	// and our internal listener goroutine to stop.
	close(d.doneC)

	// Close our connection. This should cause our listener goroutine to break
	// out of any blocking read calls.
	err := d.conn.Close()

	// Wait for our listener goroutine to complete.
	<-d.listenDoneC

	// Update monitoring information.
	d.monitoring.Update(d)

	return err
}

// UpdateHeaders sets the base discovery headers to use for this device.
//
// These headers will be updated internally to include the local device address
// information.
//
// UpdateHeaders must be called at least once before DiscoveryHeaders is
// invoked, ideally at setup.
func (d *Local) UpdateHeaders(dh *protocol.DiscoveryHeaders) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Clone the provided headers.
	d.dh = dh.Clone()

	// Fill in our address and port.
	d.dh.SetIP4Address(d.addr.IP)
	d.dh.PixelPusher.MyPort = uint16(d.addr.Port)

	// Update monitoring information.
	d.monitoring.Update(d)
}

// String implements D.
func (d *Local) String() string { return fmt.Sprintf("Local{%s}", d.addr.String()) }

// ID implements D.
func (d *Local) ID() string { return d.DeviceID }

// Ordinal implements D.
//
// The Local is not part of any ordinal group.
func (d *Local) Ordinal() Ordinal { return InvalidOrdinal() }

// Sender implements D.
func (d *Local) Sender() (Sender, error) {
	return nil, errors.New("local device Sender is not supported")
}

// DiscoveryHeaders implements D.
func (d *Local) DiscoveryHeaders() *protocol.DiscoveryHeaders {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.dh == nil {
		return &protocol.DiscoveryHeaders{}
	}
	return d.dh
}

// DoneC implements D.
func (d *Local) DoneC() <-chan struct{} { return d.doneC }

// Addr implements D.
//
// Addr will always be a *net.UDPAddr.
func (d *Local) Addr() net.Addr { return d.addr }

// Info implements D.
func (d *Local) Info() Info { return Info{} }

// ListenForPackets listens on D's address for packets, sending all received
// packets to d's callback.
func (d *Local) listenForPackets() {
	defer close(d.listenDoneC)

	for {
		// If we've been closed, then we're done.
		select {
		case <-d.doneC:
			return
		default:
		}

		// Listen for an incoming packet.
		buf := d.packetPool.Get()
		size, _, _, addr, err := d.conn.ReadMsgUDP(buf.Bytes(), nil)
		if err != nil {
			// TODO: Log error?
			continue
		}
		buf.Truncate(size)

		d.logger.Debugf("Received packet from %s (%d byte(s)) on %s:\n%s",
			addr, size, d.DeviceID, fmtutil.Hex(buf.Bytes()))

		d.dispatchPacketToCallback(buf)
	}
}

func (d *Local) dispatchPacketToCallback(buf *bufferpool.Buffer) {
	defer buf.Release()

	defer func() {
		if err := recover(); err != nil {
			d.logger.Warnf("Dropping panic in callback: %s", err)
		}
	}()
	d.OnPacketData(buf)
}
