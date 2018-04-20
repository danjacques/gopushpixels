// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package protocol

import (
	"bytes"
	"net"
	"testing"

	"github.com/danjacques/gopushpixels/protocol/pixelpusher"
	"github.com/danjacques/gopushpixels/protocol/protocoltest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Device Header", func() {
	var dh *DeviceHeader
	BeforeEach(func() {
		dh = &DeviceHeader{
			MacAddress:       [6]byte{0x90, 0xA0, 0xB0, 0xC0, 0xD0, 0xE0},
			IPAddress:        [4]byte{127, 0, 0, 1},
			DeviceType:       EtherDreamDeviceType,
			ProtocolVersion:  DefaultProtocolVersion,
			VendorID:         0x1337,
			ProductID:        0xFACE,
			HardwareRevision: 0x1234,
			SoftwareRevision: 0,
			LinkSpeed:        0x89ABCDEF,
		}
	})

	Context("the hardware address", func() {
		mustParseMAC := func(v string) net.HardwareAddr {
			addr, err := net.ParseMAC(v)
			if err != nil {
				panic(err)
			}
			return addr
		}

		It("can be converted to Go type", func() {
			addr := dh.HardwareAddr()
			Expect(addr).To(Equal(mustParseMAC("90:A0:B0:C0:D0:E0")))
		})

		It("can be set to a Go type", func() {
			v, err := net.ParseMAC("F0:0D:FA:CE:BE:EF")
			Expect(err).ToNot(HaveOccurred())

			dh.SetHardwareAddr(v)
			Expect(dh.HardwareAddr()).To(Equal(net.HardwareAddr{0xF0, 0x0D, 0xFA, 0xCE, 0xBE, 0xEF}))
		})
	})

	Context("the IP address", func() {
		It("can be converted to Go type", func() {
			addr := dh.IP4Address()
			Expect(addr).To(Equal(net.ParseIP("127.0.0.1")))
		})

		It("can be set to a Go type", func() {
			dh.SetIP4Address(net.ParseIP("10.0.0.1"))
			Expect(dh.IPAddress).To(Equal([4]byte{10, 0, 0, 1}))
		})
	})

	Context("a discovery header", func() {
		var discovery *DiscoveryHeaders
		BeforeEach(func() {
			discovery = &DiscoveryHeaders{
				DeviceHeader: *dh,
			}
		})

		It("will report a zero strip count", func() {
			count := discovery.NumStrips()
			Expect(count).To(BeZero())
		})

		It("will report a zero pixel count", func() {
			count := discovery.NumPixels()
			Expect(count).To(BeZero())
		})
	})
})

var _ = Describe("PixelPusher Discovery", func() {
	content := protocoltest.PixelPusherDiscoveryPacket()

	raw := bytes.Join([][]byte{
		content,
		{0x55, 0xAA, 0x55, 0xAA}, // (Extra)
	}, nil)

	dh := DiscoveryHeaders{
		DeviceHeader: DeviceHeader{
			MacAddress:       [6]byte{0xFA, 0xCE, 0xFE, 0xED, 0x70, 0xAD},
			IPAddress:        [4]byte{0x0A, 0x00, 0x00, 0x01},
			DeviceType:       PixelPusherDeviceType,
			ProtocolVersion:  DefaultProtocolVersion,
			VendorID:         0x1337,
			ProductID:        0xDAB5,
			HardwareRevision: 0xCCDD,
			SoftwareRevision: 130,
			LinkSpeed:        0x12345678,
		},

		PixelPusher: &pixelpusher.Device{
			DeviceHeader: pixelpusher.DeviceHeader{
				StripsAttached:     6,
				MaxStripsPerPacket: 2,
				PixelsPerStrip:     128,
				UpdatePeriod:       0x10111213,
				PowerTotal:         0x20212223,
				DeltaSequence:      0x30313233,
				ControllerOrdinal:  0x40414243,
				GroupOrdinal:       0x50515253,
				ArtNetUniverse:     0x6061,
				ArtNetChannel:      0x7071,
			},
			DeviceHeaderExt101: pixelpusher.DeviceHeaderExt101{
				MyPort: 0xFACE,
			},
			DeviceHeaderExt109: pixelpusher.DeviceHeaderExt109{
				StripFlags: []pixelpusher.StripFlags{0x70, 0x71, 0x72, 0x73, 0x74, 0x75},
			},
			DeviceHeaderExt117: pixelpusher.DeviceHeaderExt117{
				PusherFlags: 0x55667788,
				Segments:    0x11223344,
				PowerDomain: 0xAABBCCDD,
			},
			Extra: []byte{0x55, 0xAA, 0x55, 0xAA},
		},
	}

	Context("reading a discovery packet", func() {
		It("parses the header data properly", func() {
			d, err := ParseDiscoveryHeaders(raw)
			Expect(err).ToNot(HaveOccurred())
			Expect(d).To(Equal(&dh))
		})

		It("can write the header data", func() {
			var buf bytes.Buffer
			err := dh.WritePacket(&buf)
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.Bytes()).To(Equal(content))
		})

		It("can generate a UDP address", func() {
			addr := dh.Addr()
			Expect(addr).To(Equal(&net.UDPAddr{
				IP:   net.ParseIP("10.0.0.1"),
				Port: 0xFACE,
			}))
		})
	})

	It("can generate an accurate strip count", func() {
		count := dh.NumStrips()
		Expect(count).To(Equal(6))
	})

	It("can generate an accurate pixel count", func() {
		count := dh.NumPixels()
		Expect(count).To(Equal(128 * 6))
	})

	It("can generate a populated PacketReader", func() {
		pr, err := dh.PacketReader()
		Expect(err).ToNot(HaveOccurred())

		Expect(pr.PixelPusher).ToNot(BeNil())
		Expect(pr.PixelPusher.PixelsPerStrip).To(Equal(128))
		Expect(pr.PixelPusher.StripFlags).To(HaveLen(6))
		for i, sf := range pr.PixelPusher.StripFlags {
			Expect(sf).To(Equal(dh.PixelPusher.StripFlags[i]), "strip flag #%d", i)
		}
	})

	It("can generate a populated PacketStream", func() {
		ps, err := dh.PacketStream()
		Expect(err).ToNot(HaveOccurred())

		Expect(ps.PixelPusher).ToNot(BeNil())
		Expect(ps.PixelPusher.MaxStripsPerPacket).To(BeEquivalentTo(2))
		Expect(ps.PixelPusher.PixelsPerStrip).To(BeEquivalentTo(128))
		Expect(ps.PixelPusher.FixedSize).To(BeEquivalentTo(0))
	})
})

func TestProtocol(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Protocol Tests")
}
