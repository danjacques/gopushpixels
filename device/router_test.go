// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"github.com/danjacques/gopushpixels/protocol"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Router", func() {
	var r *Router
	BeforeEach(func() {
		r = &Router{
			Registry: &Registry{},
		}
	})
	AfterEach(func() {
		r.Shutdown()
	})

	It("will return ErrNoRoute for unregistered devices", func() {
		err := r.Route(InvalidOrdinal(), "nonexist", nil)
		Expect(err).To(Equal(ErrNoRoute))
	})

	Context("with two registered devices", func() {
		var d0, d1 *testD

		BeforeEach(func() {
			d0 = makeTestD("foo")
			d0.ordinal.Group = 1
			r.Registry.Add(d0)

			d1 = makeTestD("bar")
			d1.ordinal.Group = 2
			r.Registry.Add(d1)
		})
		AfterEach(func() {
			d0.markDone()
			d1.markDone()
		})

		It("can route to both devices by ID", func() {
			pktFoo := &protocol.Packet{}
			err := r.Route(InvalidOrdinal(), "foo", pktFoo)
			Expect(err).ToNot(HaveOccurred())

			pktBar := &protocol.Packet{}
			err = r.Route(InvalidOrdinal(), "bar", pktBar)
			Expect(err).ToNot(HaveOccurred())

			Expect(d0.packets).To(ConsistOf(pktFoo))
			Expect(d1.packets).To(ConsistOf(pktFoo))
		})

		It("can route to both devices by Ordinal", func() {
			// Group 1 == foo
			pktFoo := &protocol.Packet{}
			err := r.Route(Ordinal{Group: 1}, "nonexist", pktFoo)
			Expect(err).ToNot(HaveOccurred())

			// Group 1 == bar
			pktBar := &protocol.Packet{}
			err = r.Route(Ordinal{Group: 2}, "nonexist", pktBar)
			Expect(err).ToNot(HaveOccurred())

			Expect(d0.packets).To(ConsistOf(pktFoo))
			Expect(d1.packets).To(ConsistOf(pktFoo))
		})

		It("routes to Ordinal before ID", func() {
			// Group 1 == foo, but we use "bar" ID.
			pktFoo := &protocol.Packet{}
			err := r.Route(Ordinal{Group: 1}, "bar", pktFoo)
			Expect(err).ToNot(HaveOccurred())

			// Group 2 == bar, but we use "foo" ID.
			pktBar := &protocol.Packet{}
			err = r.Route(Ordinal{Group: 2}, "foo", pktBar)
			Expect(err).ToNot(HaveOccurred())

			Expect(d0.packets).To(ConsistOf(pktFoo))
			Expect(d1.packets).To(ConsistOf(pktFoo))
		})

		Context("when connected to a Listener", func() {
			type capturedPacket struct {
				d   D
				pkt *protocol.Packet
			}

			var l Listener
			var packets []capturedPacket

			BeforeEach(func() {
				packets = nil
				l = ListenerFunc(func(d D, pkt *protocol.Packet) {
					packets = append(packets, capturedPacket{d, pkt})
				})
				r.AddListener(l)
			})
			AfterEach(func() { r.RemoveListener(l) })

			It("can send to a Listener", func() {
				pkt := &protocol.Packet{}
				err := r.Route(InvalidOrdinal(), "foo", pkt)
				Expect(err).ToNot(HaveOccurred())

				By("the packet was routed")
				Expect(d0.packets).To(ConsistOf(pkt))

				By("the listener received the packet")
				Expect(packets).To(Equal([]capturedPacket{
					{d: d0, pkt: pkt},
				}))
			})

			It("when unroutable, the Listener does not receive the packet", func() {
				pkt := &protocol.Packet{}
				err := r.Route(InvalidOrdinal(), "nonexist", pkt)
				Expect(err).To(Equal(ErrNoRoute))

				By("the listener received the packet")
				Expect(packets).To(BeEmpty())
			})
		})
	})
})
