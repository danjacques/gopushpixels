// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Discovery Headers", func() {
	// A full discovery packet.
	base := []byte{
		0x01,
		0x02,
		0x03, 0x00,
		0x13, 0x12, 0x11, 0x10,
		0x23, 0x22, 0x21, 0x20,
		0x33, 0x32, 0x31, 0x30,
		0x43, 0x42, 0x41, 0x40,
		0x53, 0x52, 0x51, 0x50,
		0x61, 0x60,
		0x71, 0x70,
	}

	ext101 := []byte{
		0xCE, 0xFA,
		0x00, 0x00,
	}

	ext109 := []byte{
		0x0F, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	ext117 := []byte{
		0x00, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF,
		0xA3, 0xA2, 0xA1, 0xA0,
		0xB3, 0xB2, 0xB1, 0xB0,
	}

	extra := []byte{0xF0, 0xF1, 0xF2, 0xF3, 0xF4}

	fullDiscovery := bytes.Join([][]byte{
		base,
		ext101,
		ext109,
		ext117,
		extra,
	}, nil)

	Context("reading for different software versions", func() {
		It("can read for version 100", func() {
			d, err := ReadDevice(bytes.NewReader(fullDiscovery), 100)
			Expect(err).ToNot(HaveOccurred())

			Expect(d).To(Equal(&Device{
				DeviceHeader: DeviceHeader{
					StripsAttached:     1,
					MaxStripsPerPacket: 2,
					PixelsPerStrip:     3,
					UpdatePeriod:       0x10111213,
					PowerTotal:         0x20212223,
					DeltaSequence:      0x30313233,
					ControllerOrdinal:  0x40414243,
					GroupOrdinal:       0x50515253,
					ArtNetUniverse:     0x6061,
					ArtNetChannel:      0x7071,
				},
				DeviceHeaderExt101: DeviceHeaderExt101{
					MyPort: DefaultPort,
				},
				DeviceHeaderExt109: DeviceHeaderExt109{
					StripFlags: []StripFlags{0},
				},
				DeviceHeaderExt117: DeviceHeaderExt117{
					PusherFlags: 0,
					Segments:    0,
					PowerDomain: 0,
				},
				Extra: fullDiscovery[28:],
			}))
		})

		It("can read for version 101", func() {
			d, err := ReadDevice(bytes.NewReader(fullDiscovery), 101)
			Expect(err).ToNot(HaveOccurred())

			Expect(d).To(Equal(&Device{
				DeviceHeader: DeviceHeader{
					StripsAttached:     1,
					MaxStripsPerPacket: 2,
					PixelsPerStrip:     3,
					UpdatePeriod:       0x10111213,
					PowerTotal:         0x20212223,
					DeltaSequence:      0x30313233,
					ControllerOrdinal:  0x40414243,
					GroupOrdinal:       0x50515253,
					ArtNetUniverse:     0x6061,
					ArtNetChannel:      0x7071,
				},
				DeviceHeaderExt101: DeviceHeaderExt101{
					MyPort: 0xFACE,
				},
				DeviceHeaderExt109: DeviceHeaderExt109{
					StripFlags: []StripFlags{0},
				},
				DeviceHeaderExt117: DeviceHeaderExt117{
					PusherFlags: 0,
					Segments:    0,
					PowerDomain: 0,
				},
				Extra: fullDiscovery[32:],
			}))
		})

		It("can read for version 109", func() {
			d, err := ReadDevice(bytes.NewReader(fullDiscovery), 109)
			Expect(err).ToNot(HaveOccurred())

			Expect(d).To(Equal(&Device{
				DeviceHeader: DeviceHeader{
					StripsAttached:     1,
					MaxStripsPerPacket: 2,
					PixelsPerStrip:     3,
					UpdatePeriod:       0x10111213,
					PowerTotal:         0x20212223,
					DeltaSequence:      0x30313233,
					ControllerOrdinal:  0x40414243,
					GroupOrdinal:       0x50515253,
					ArtNetUniverse:     0x6061,
					ArtNetChannel:      0x7071,
				},
				DeviceHeaderExt101: DeviceHeaderExt101{
					MyPort: 0xFACE,
				},
				DeviceHeaderExt109: DeviceHeaderExt109{
					StripFlags: []StripFlags{0x0F},
				},
				DeviceHeaderExt117: DeviceHeaderExt117{
					PusherFlags: 0,
					Segments:    0,
					PowerDomain: 0,
				},
				Extra: fullDiscovery[40:],
			}))
		})

		It("can read for version 117", func() {
			d, err := ReadDevice(bytes.NewReader(fullDiscovery), 117)
			Expect(err).ToNot(HaveOccurred())

			Expect(d).To(Equal(&Device{
				DeviceHeader: DeviceHeader{
					StripsAttached:     1,
					MaxStripsPerPacket: 2,
					PixelsPerStrip:     3,
					UpdatePeriod:       0x10111213,
					PowerTotal:         0x20212223,
					DeltaSequence:      0x30313233,
					ControllerOrdinal:  0x40414243,
					GroupOrdinal:       0x50515253,
					ArtNetUniverse:     0x6061,
					ArtNetChannel:      0x7071,
				},
				DeviceHeaderExt101: DeviceHeaderExt101{
					MyPort: 0xFACE,
				},
				DeviceHeaderExt109: DeviceHeaderExt109{
					StripFlags: []StripFlags{0x0F},
				},
				DeviceHeaderExt117: DeviceHeaderExt117{
					PusherFlags: 0xFFFFFFFF,
					Segments:    0xA0A1A2A3,
					PowerDomain: 0xB0B1B2B3,
				},
				Extra: fullDiscovery[54:],
			}))
		})
	})

	Context("writing for different software versions", func() {
		d := Device{
			DeviceHeader: DeviceHeader{
				StripsAttached:     1,
				MaxStripsPerPacket: 2,
				PixelsPerStrip:     3,
				UpdatePeriod:       0x10111213,
				PowerTotal:         0x20212223,
				DeltaSequence:      0x30313233,
				ControllerOrdinal:  0x40414243,
				GroupOrdinal:       0x50515253,
				ArtNetUniverse:     0x6061,
				ArtNetChannel:      0x7071,
			},
			DeviceHeaderExt101: DeviceHeaderExt101{
				MyPort: 0xFACE,
			},
			DeviceHeaderExt109: DeviceHeaderExt109{
				StripFlags: []StripFlags{0x0F},
			},
			DeviceHeaderExt117: DeviceHeaderExt117{
				PusherFlags: 0xFFFFFFFF,
				Segments:    0xA0A1A2A3,
				PowerDomain: 0xB0B1B2B3,
			},
		}

		var buf bytes.Buffer
		BeforeEach(func() {
			buf.Reset()
		})

		It("can write for version 100", func() {
			err := d.Write(&buf, 100)
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.Bytes()).To(Equal(bytes.Join([][]byte{
				base,
			}, nil)))
		})

		It("can write for version 101", func() {
			err := d.Write(&buf, 101)
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.Bytes()).To(Equal(bytes.Join([][]byte{
				base,
				ext101,
			}, nil)))
		})

		It("can write for version 109", func() {
			err := d.Write(&buf, 109)
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.Bytes()).To(Equal(bytes.Join([][]byte{
				base,
				ext101,
				ext109,
			}, nil)))
		})

		It("can write for version 117", func() {
			err := d.Write(&buf, 117)
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.Bytes()).To(Equal(bytes.Join([][]byte{
				base,
				ext101,
				ext109,
				ext117,
			}, nil)))
		})
	})

	Context("given discovery headers", func() {
		var d *Device
		BeforeEach(func() {
			d = &Device{
				DeviceHeader: DeviceHeader{
					StripsAttached:     2,
					MaxStripsPerPacket: 2,
					PixelsPerStrip:     3,
				},
				DeviceHeaderExt101: DeviceHeaderExt101{
					MyPort: 0xFACE,
				},
				DeviceHeaderExt109: DeviceHeaderExt109{
					StripFlags: []StripFlags{0x0F, 0x0F},
				},
			}
		})

		It("can be cloned, and is independent", func() {
			clone := d.Clone()
			d.MyPort = 0
			d.StripFlags[1] = 0

			Expect(clone.MyPort).To(BeEquivalentTo(0xFACE))
			Expect(clone.StripFlags).To(Equal([]StripFlags{0x0F, 0x0F}))
		})

		Context("when PFLAG_FIXEDSIZE is clear", func() {
			BeforeEach(func() {
				d.PusherFlags = 0
			})

			It("will return a fixed size of 0", func() {
				Expect(d.FixedSize()).To(Equal(0))
			})
		})

		Context("when PFLAG_FIXEDSIZE is set", func() {
			BeforeEach(func() {
				d.PusherFlags = PFlagFixedSize
			})

			It("will return a fixed size", func() {
				Expect(d.FixedSize()).To(Equal(24))
			})
		})

		It("can return a configured PacketReader", func() {
			Expect(d.PacketReader()).To(Equal(&PacketReader{
				PixelsPerStrip: 3,
				StripFlags:     []StripFlags{0x0F, 0x0F},
			}))
		})

		It("can return a configured PacketStream", func() {
			Expect(d.PacketStream()).To(Equal(&PacketStream{
				MaxStripsPerPacket: 2,
				PixelsPerStrip:     3,
				FixedSize:          0,
			}))
		})
	})
})
