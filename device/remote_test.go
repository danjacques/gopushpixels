// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"net"
	"sync"
	"time"

	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/protocol/pixelpusher"
	"github.com/danjacques/gopushpixels/support/network"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Remote Device (Connected)", func() {
	// Open up a local UDP connection to pretend to be the remote system.
	var rc *remoteConn
	BeforeEach(func() {
		// The buffer size is the amount of data that the packet channel can buffer.
		rc = newRemoteConn(8)

		err := rc.add("orig")
		Expect(err).ToNot(HaveOccurred())
	})
	AfterEach(func() { rc.closeAll() })

	// Create discovery headers for a PixelPusher.
	//
	// We choose this device because it can specify its own port.
	var dh *protocol.DiscoveryHeaders
	BeforeEach(func() {
		dh = &protocol.DiscoveryHeaders{
			DeviceHeader: protocol.DeviceHeader{
				DeviceType: protocol.PixelPusherDeviceType,
				MacAddress: [6]byte{0xf0, 0x0d, 0xfa, 0xce, 0xd0, 0x65},
			},
			PixelPusher: &pixelpusher.Device{},
		}
		dh.PixelPusher.GroupOrdinal = 4
		dh.PixelPusher.ControllerOrdinal = 8

		// Configure these headers to broadcast for "orig".
		rc.setDiscoveryHeaders("orig", dh)
	})

	// Initialize our remote device for this round.
	var r *Remote
	BeforeEach(func() {
		r = MakeRemote("test device", dh)
	})
	AfterEach(func() {
		r.MarkDone()
	})

	It("reports its basic fields", func() {
		By("id")
		Expect(r.ID()).To(Equal("test device"))

		By("discovery headers")
		Expect(r.DiscoveryHeaders()).To(Equal(dh))

		By("ordinal")
		ord := r.Ordinal()
		Expect(ord).To(Equal(Ordinal{
			Group:      4,
			Controller: 8,
		}))

		By("string")
		Expect(r.String()).To(MatchRegexp(`"test device" @.+ \(PIXELPUSHER\)`))
	})

	Context("with a Sender", func() {
		var s Sender
		BeforeEach(func() {
			var err error
			s, err = r.Sender()
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func(done Done) {
			defer close(done)

			if s != nil {
				err := s.Close()
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("can send packets", func(done Done) {
			defer close(done)

			By("first packet")
			err := s.SendDatagram([]byte("Her Glorious Majesty, Packet the First"))
			Expect(err).ToNot(HaveOccurred())

			By("second packet")
			err = s.SendDatagram([]byte("The Usurper, Packet II"))
			Expect(err).ToNot(HaveOccurred())

			By("collect")
			Expect(<-rc.packetC).To(Equal(&remoteConnPacket{
				id:  "orig",
				pkt: []byte("Her Glorious Majesty, Packet the First"),
			}))
			Expect(<-rc.packetC).To(Equal(&remoteConnPacket{
				id:  "orig",
				pkt: []byte("The Usurper, Packet II"),
			}))

			By("reports that packets have been sent")
			info := r.Info()
			Expect(info.BytesSent).To(BeNumerically(">", 0))
			Expect(info.PacketsSent).To(BeEquivalentTo(2))
		})

		Context("when the port dynamically changes", func() {
			var ndh *protocol.DiscoveryHeaders

			BeforeEach(func() {
				err := rc.add("mod")
				Expect(err).ToNot(HaveOccurred())

				ndh = dh.Clone()
				rc.setDiscoveryHeaders("mod", ndh)
			})

			It("can dynamically change ports when a new header is sent", func(done Done) {
				defer close(done)

				By("receive a packet on original port")
				err := s.SendDatagram([]byte("ohai there!"))
				Expect(err).ToNot(HaveOccurred())
				Expect(<-rc.packetC).To(Equal(&remoteConnPacket{
					id:  "orig",
					pkt: []byte("ohai there!"),
				}))

				By("change port")
				r.UpdateHeaders(time.Now(), ndh)

				By("writes a packet to the new port")
				err = s.SendDatagram([]byte("changed port, u keep up?"))
				Expect(err).ToNot(HaveOccurred())
				Expect(<-rc.packetC).To(Equal(&remoteConnPacket{
					id:  "mod",
					pkt: []byte("changed port, u keep up?"),
				}))

				By("reports that packets have been sent")
				info := r.Info()
				Expect(info.BytesSent).To(BeNumerically(">", 0))
				Expect(info.PacketsSent).To(BeEquivalentTo(2))
			})
		})
	})

	Context("when marked Done", func() {
		BeforeEach(func() { r.MarkDone() })

		It("reports itself as Done", func() {
			Expect(IsDone(r)).To(BeTrue())
		})
	})
})

var _ = Describe("Remote Stub", func() {
})

// remoteConn manages labelled local UDP connections and captures/consumes
// packets sent to them.
//
// This is probably overkill, but is a nice facility to have, and is one of the
// (ideally) few things that is actually bound to a real system resource.
type remoteConn struct {
	packetC chan *remoteConnPacket
	conns   map[string]*net.UDPConn
	wg      sync.WaitGroup
}

func newRemoteConn(bufferSize int) *remoteConn {
	return &remoteConn{
		packetC: make(chan *remoteConnPacket, bufferSize),
	}
}

func (rc *remoteConn) add(id string) error {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP: net.ParseIP("127.0.0.1"),
	})
	if err != nil {
		return err
	}

	if rc.conns == nil {
		rc.conns = make(map[string]*net.UDPConn)
	}
	rc.conns[id] = conn

	rc.wg.Add(1)
	go rc.listenOn(id, conn)
	return nil
}

func (rc *remoteConn) setDiscoveryHeaders(id string, dh *protocol.DiscoveryHeaders) {
	conn := rc.conns[id]
	addr := conn.LocalAddr().(*net.UDPAddr)

	dh.SetIP4Address(addr.IP)
	dh.PixelPusher.MyPort = uint16(addr.Port)
}

func (rc *remoteConn) closeAll() {
	for _, conn := range rc.conns {
		_ = conn.Close()
	}
	rc.conns = nil
}

func (rc *remoteConn) listenOn(id string, conn *net.UDPConn) {
	defer rc.wg.Done()

	for {
		buf := make([]byte, network.MaxUDPSize)
		amt, _, _, _, err := conn.ReadMsgUDP(buf, nil)
		if err != nil {
			return
		}

		rc.packetC <- &remoteConnPacket{id, buf[:amt]}
	}
}

type remoteConnPacket struct {
	id  string
	pkt []byte
}
