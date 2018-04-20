// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package discovery

import (
	"context"
	"net"

	"github.com/danjacques/gopushpixels/protocol/protocoltest"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockListenerConnection struct {
	DataC      chan []byte
	ClientAddr *net.UDPAddr

	err         error
	readSignalC chan struct{}
	closed      bool
}

func (mlc *mockListenerConnection) Close() error {
	if mlc.closed {
		return errors.New("already closed")
	}

	close(mlc.DataC)
	mlc.closed = true
	return mlc.err
}

func (mlc *mockListenerConnection) SetReadBuffer(size int) error { return mlc.err }

func (mlc *mockListenerConnection) LocalAddr() net.Addr {
	return &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 1337,
	}
}

func (mlc *mockListenerConnection) ReadFromUDP(buf []byte) (int, *net.UDPAddr, error) {
	//' If someone has registered to be notified when a read begins, notify.
	if mlc.readSignalC != nil {
		mlc.readSignalC <- struct{}{}
	}

	// Instrumented error.
	if mlc.err != nil {
		return 0, nil, mlc.err
	}

	d, ok := <-mlc.DataC
	if !ok {
		return 0, nil, errors.New("connection closed")
	}

	size := copy(buf, d)
	return size, mlc.ClientAddr, nil
}

var _ = Describe("Listener", func() {
	// A generic PixelPusher discovery packet.
	pp := protocoltest.PixelPusherDiscoveryPacket()

	var conn *mockListenerConnection
	BeforeEach(func() {
		conn = &mockListenerConnection{
			DataC: make(chan []byte, 1),
			ClientAddr: &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.2"),
				Port: 2468,
			},
		}
	})

	Context("once started", func() {
		var l *Listener
		BeforeEach(func() {
			l = &Listener{}

			err := l.startInternal(conn)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			if l != nil {
				_ = l.Close()
			}
		})

		It("can close immediately", func() {
			err := l.Close()
			Expect(err).ToNot(HaveOccurred())

			l = nil // Don't double-close on cleanup.
		})

		It("can read packets", func(done Done) {
			defer close(done)

			c, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()

			conn.DataC <- pp
			dh, err := l.Accept(c)
			Expect(err).ToNot(HaveOccurred())
			Expect(dh).ToNot(BeNil())
		}, 1)

		It("will cancel a read if the Context is cancelled", func(done Done) {
			defer close(done)

			c, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()

			// We will ask our mock to notify when a read begins.
			readSignalC := make(chan struct{})
			conn.readSignalC = readSignalC

			// Begin listening for a packet. We never put data in DataC, though, so
			// this will never receive data.
			errC := make(chan error)
			go func() {
				_, err := l.Accept(c)
				errC <- err
			}()

			// Wait for a read to start.
			<-readSignalC

			// Cancel our Context.
			cancelFunc()

			// Ensure that we got an error.
			Expect(<-errC).To(Equal(context.Canceled))
		})

		It("will fail if our connection returns an error", func(done Done) {
			defer close(done)

			// Reads will return our instrumented error.
			conn.err = errors.New("test error")

			_, err := l.Accept(context.Background())
			Expect(err).To(MatchError("test error"))
		})
	})
})
