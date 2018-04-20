// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"sync"

	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/logging"
	"github.com/danjacques/gopushpixels/support/network"
)

// packetDispatcher is a higher-level construct that sends packets to a device.
//
// packetDispatcher must be safe for concurrent use.
//
// TODO: Improvements and optimizations, such as:
// - Better PacketConnection batching of burst requests.
// - Possibly drop packets if the device is reporting that we're exceeding its
//   network speed (see Java code for an example of how this looks).
type packetDispatcher struct {
	// d is the underlying device.
	d D

	// logger is the logger to use. It must not be nil.
	logger logging.L

	// onShutdown is called when this packet dispatcher is shutdown. It is passed
	// a pointer to the dispatcher instance that is being shut down.
	onShutdown func(*packetDispatcher)

	// shutdownC is a signal to notify that this dispatcher has been shut down.
	shutdownC chan struct{}
	// Used to prevent us from shutting down more than once.
	isShutdown bool

	// senderMu protects sender.
	senderMu sync.Mutex
	// ds is the datagram sender to use for dispatch.
	//
	// Users should access this via the protective accessor, withSender.
	sender network.DatagramSender

	// mu protects the remainder of the dispatcher state.
	mu sync.Mutex
	// stream is the underlying packet stream.
	stream *protocol.PacketStream
	// refs is the current reference count. If this hits zero, we will shutdown.
	refs int64
}

// Start starts the packet dispatcher's execution.
//
// It must be called before using any other methods, and only once.
func (pd *packetDispatcher) RetainAndStart() error {
	// Create a PacketStream for this device.
	dh := pd.d.DiscoveryHeaders()
	stream, err := dh.PacketStream()
	if err != nil {
		return err
	}

	pd.stream = stream
	pd.shutdownC = make(chan struct{})
	pd.refs = 1

	// Automatically shutdown when our base Device is Done.
	go pd.shutdownWhenDeviceIsDone()

	return nil
}

func (pd *packetDispatcher) Retain() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.refs++
}

// Release releases a single reference to the dispatcher.
//
// If the reference count hits zero, the dispatcher will shutdown before
// returning.
func (pd *packetDispatcher) Release() error {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.refs--
	if pd.refs > 0 {
		return nil
	}
	return pd.shutdownLocked()
}

func (pd *packetDispatcher) shutdownLocked() error {
	// Have we already been shutdown?
	if pd.isShutdown {
		return nil
	}

	// Invoke our shutdown callback.
	if pd.onShutdown != nil {
		pd.onShutdown(pd)
	}

	// Close our underlying connection.
	err := pd.withSender(func(ds network.DatagramSender) error {
		if err := pd.stream.Flush(ds); err != nil {
			pd.logger.Warnf("Failed to send a final flush: %s", err)
		}
		return ds.Close()
	})

	// Mark that we're shutdown.
	close(pd.shutdownC)
	pd.isShutdown = true

	return err
}

// shutdownWhenDeviceIsDone is run in its own goroutine. It monitors terminal
// conditions and shuts down the dispatcher when one is observed.
func (pd *packetDispatcher) shutdownWhenDeviceIsDone() {
	// Block until either this Dispatcher or its host Device are done.
	select {
	case <-pd.d.DoneC():
		pd.mu.Lock()
		defer pd.mu.Unlock()
		_ = pd.shutdownLocked()

	case <-pd.shutdownC:
		// Already shutdown, quit the goroutine.
	}
}

// MaxDatagramSize implements Sender via network.DatagramSender.
//
// NOTE: This will not cause a lock conflict when sending packets via our
// PacketStream because it gets a direct internal sender reference, and doesn't
// operate on the remoteSender.
func (rs *remoteSender) MaxDatagramSize() (v int) {
	_ = rs.withSender(func(ds network.DatagramSender) error {
		v = ds.MaxDatagramSize()
		return nil
	})
	return
}

// SendDatagram implements Sender via network.DatagramSender.
//
// This will acquire an exclusive lock to the underlying connection and send
// the datagram.
func (pd *packetDispatcher) SendDatagram(data []byte) error {
	return pd.withSender(func(sender network.DatagramSender) error {
		return sender.SendDatagram(data)
	})
}

// SendPacket implements Sender.
//
// SendPacket sends packet through the dispatcher state, blocking until the
// packet has been successfully sent.
func (pd *packetDispatcher) SendPacket(packet *protocol.Packet) error {
	// Take out a lock on our PacketStream.
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Send the packet immediately.
	//
	// TODO: It's possible that packets from multiple sources could be optimally
	// combined in the same datagram. Consider kicking off a goroutine/timer to
	// do the actual sending after allowing smoe time period for batching?
	return pd.withSender(func(ds network.DatagramSender) error {
		if err := pd.stream.Send(ds, packet); err != nil {
			return err
		}
		return pd.stream.Flush(ds)
	})
}

func (pd *packetDispatcher) withSender(fn func(network.DatagramSender) error) error {
	pd.senderMu.Lock()
	defer pd.senderMu.Unlock()
	return fn(pd.sender)
}
