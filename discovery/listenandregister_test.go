// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package discovery

import (
	"context"
	"net"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol/protocoltest"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ListenAndRegister", func() {
	var (
		conn *mockListenerConnection
		l    *Listener
		reg  *Registry
	)
	BeforeEach(func() {
		conn = &mockListenerConnection{
			DataC: make(chan []byte, 1),
			ClientAddr: &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.2"),
				Port: 2468,
			},
		}
		l = &Listener{}
		reg = &Registry{}
	})
	AfterEach(func() {
		_ = l.Close()
	})

	Context("with the Listener listening", func() {
		BeforeEach(func() {
			err := l.startInternal(conn)
			Expect(err).ToNot(HaveOccurred())
		})

		It("can listen and register connections", func() {
			c := context.Background()
			conn.DataC <- protocoltest.PixelPusherDiscoveryPacket()

			var gotD device.D
			err := ListenAndRegister(c, l, reg, func(d device.D) error {
				gotD = d

				return l.Close()
			})
			Expect(err).To(MatchError("the Listener is not active"))
			Expect(gotD).ToNot(BeNil())
		})
	})

	It("will terminate if Context is cancelled", func(done Done) {
		defer close(done)

		conn.readSignalC = make(chan struct{})
		err := l.startInternal(conn)
		Expect(err).ToNot(HaveOccurred())

		c, cancelFunc := context.WithCancel(context.Background())
		defer cancelFunc()

		// Wait for our read to happen, then cancel.
		go func() {
			<-conn.readSignalC
			cancelFunc()
		}()

		err = ListenAndRegister(c, l, reg, nil)
		Expect(err).To(Equal(context.Canceled))
	})

	It("will terminate if the callback returns an error", func() {
		conn.DataC <- protocoltest.PixelPusherDiscoveryPacket()
		err := l.startInternal(conn)
		Expect(err).ToNot(HaveOccurred())

		c := context.Background()

		testErr := errors.New("test error")
		err = ListenAndRegister(c, l, reg, func(_ device.D) error { return testErr })
		Expect(err).To(Equal(testErr))
	})
})
