// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	"bytes"

	"github.com/danjacques/gopushpixels/pixel"
	"github.com/danjacques/gopushpixels/support/byteslicereader"
	"github.com/danjacques/gopushpixels/support/network"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func fillPixelBuffer(pb *pixel.Buffer, base, size int) *pixel.Buffer {
	pb.Reset(size)

	buf := pb.Bytes()
	for i := range buf {
		buf[i] = byte(i + base)
	}

	return pb
}

var _ = Describe("Packet Parsing", func() {
	// Create a packet layout with 2 pixels per strip.
	//
	// The first strip will be RGB.
	// The second strip will be RGBOW.
	pr := PacketReader{
		PixelsPerStrip: 2,
		StripFlags:     make([]StripFlags, 2),
	}
	pr.StripFlags[0].SetRGBOW(false)
	pr.StripFlags[1].SetRGBOW(true)

	It("can read a command packet", func() {
		data := bytes.Join([][]byte{
			{0x10, 0x20, 0x30, 0x40},
			CommandMagic,
			{byte(CommandReset)},
		}, nil)

		var pkt Packet
		err := pr.ReadPacket(&byteslicereader.R{Buffer: data}, &pkt)
		Expect(err).ToNot(HaveOccurred())

		Expect(pkt).To(Equal(Packet{
			ID:      0x10203040,
			Command: &ResetCommand{},
		}))
	})

	It("can read a pixel for strip #1", func() {
		data := bytes.Join([][]byte{
			{0xAA, 0xBB, 0xCC, 0xDD},
			{0x01},
			{
				1, 2, 3, 4, 4, 4, 5, 5, 5,
				6, 7, 8, 9, 9, 9, 10, 10, 10,
			},
		}, nil)

		expectedPixels := pixel.Buffer{Layout: pixel.BufferRGBOW}
		expectedPixels.Reset(2)
		expectedPixels.SetPixel(0, pixel.P{Red: 1, Green: 2, Blue: 3, Orange: 4, White: 5})
		expectedPixels.SetPixel(1, pixel.P{Red: 6, Green: 7, Blue: 8, Orange: 9, White: 10})

		var pkt Packet
		err := pr.ReadPacket(&byteslicereader.R{Buffer: data}, &pkt)
		Expect(err).ToNot(HaveOccurred())

		Expect(pkt).To(Equal(Packet{
			ID: 0xAABBCCDD,
			StripStates: []*StripState{
				{
					StripNumber: 1,
					Pixels:      expectedPixels,
				},
			},
		}))
	})

	It("can read large pixels for both strips", func() {
		pr.PixelsPerStrip = 512

		expectedPixels0 := fillPixelBuffer(&pixel.Buffer{Layout: pixel.BufferRGB}, 0, pr.PixelsPerStrip)
		expectedPixels1 := fillPixelBuffer(&pixel.Buffer{Layout: pixel.BufferRGBOW}, 0, pr.PixelsPerStrip)

		data := bytes.Join([][]byte{
			{0xAA, 0xBB, 0xCC, 0xDD},
			{0x01},
			expectedPixels1.Bytes(),
			{0x00},
			expectedPixels0.Bytes(),
		}, nil)

		var pkt Packet
		err := pr.ReadPacket(&byteslicereader.R{Buffer: data}, &pkt)
		Expect(err).ToNot(HaveOccurred())

		Expect(pkt).To(Equal(Packet{
			ID: 0xAABBCCDD,
			StripStates: []*StripState{
				{
					StripNumber: 1,
					Pixels:      *expectedPixels1,
				},
				{
					StripNumber: 0,
					Pixels:      *expectedPixels0,
				},
			},
		}))
	})
})

type mockDatagramSender struct {
	network.DatagramSender

	datagrams [][]byte
	sendErr   error

	maxDatagramSize int
}

func (mds *mockDatagramSender) SendDatagram(b []byte) error {
	if mds.sendErr != nil {
		return mds.sendErr
	}
	mds.datagrams = append(mds.datagrams, append([]byte(nil), b...))
	return nil
}

func (mds *mockDatagramSender) MaxDatagramSize() int { return mds.maxDatagramSize }

var _ = Describe("Packet Building", func() {
	var (
		ds *mockDatagramSender
		ps *PacketStream
	)

	const defaultPixelsPerStrip = 16
	BeforeEach(func() {
		ds = &mockDatagramSender{
			maxDatagramSize: 1024,
		}
		ps = &PacketStream{
			MaxStripsPerPacket: 2,
			PixelsPerStrip:     defaultPixelsPerStrip,
			FixedSize:          0,
			NextID:             0xFACE,
		}
	})

	Context("sending command packets", func() {
		It("should each command immediately", func() {
			err := ps.SendCommand(ds, &GlobalBrightnessSetCommand{Parameter: 0x1234})
			Expect(err).ToNot(HaveOccurred())

			err = ps.SendCommand(ds, &GlobalBrightnessSetCommand{Parameter: 0x5678})
			Expect(err).ToNot(HaveOccurred())

			Expect(ds.datagrams).To(HaveLen(2))
			Expect(ds.datagrams[0]).To(Equal(bytes.Join([][]byte{
				{0x00, 0x00, 0xFA, 0xCE},
				CommandMagic,
				{0x02, 0x34, 0x12},
			}, nil)))

			Expect(ds.datagrams[1]).To(Equal(bytes.Join([][]byte{
				{0x00, 0x00, 0xFA, 0xCF},
				CommandMagic,
				{0x02, 0x78, 0x56},
			}, nil)))
		})

		Context("with a fixed packet size", func() {
			// 2 is large enough for a ResetCommand (1 byte), but not large enough
			// for a GlobalBrightnessSetCommmand (3 bytes).
			BeforeEach(func() {
				ps.FixedSize = 4 + len(CommandMagic) + 2
			})

			It("will pad packets that are smaller than the fixed size", func() {
				err := ps.SendCommand(ds, &ResetCommand{})
				Expect(err).ToNot(HaveOccurred())

				Expect(ds.datagrams).To(Equal([][]byte{
					bytes.Join([][]byte{
						{0x00, 0x00, 0xFA, 0xCE},
						CommandMagic,
						{byte(CommandReset)},
						{0x00}, // Padding to FixedSize.
					}, nil),
				}))
			})

			It("will write packets that are too large", func() {
				err := ps.SendCommand(ds, &GlobalBrightnessSetCommand{Parameter: 0x1234})
				Expect(err).ToNot(HaveOccurred())

				Expect(ds.datagrams).To(Equal([][]byte{
					bytes.Join([][]byte{
						{0x00, 0x00, 0xFA, 0xCE},
						CommandMagic,
						{byte(CommandGlobalBrightnessSet)},
						{0x34, 0x12},
					}, nil),
				}))
			})
		})

		Context("when the packet can't fit in the connection", func() {
			// 1 is not large enough for a GlobalBrightnessSetCommmand packet.
			BeforeEach(func() {
				ds.maxDatagramSize = 4 + len(CommandMagic) + 1
			})

			It("will return an error on Send", func() {
				err := ps.SendCommand(ds, &GlobalBrightnessSetCommand{Parameter: 0x5678})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("sending pixel packets", func() {

		strip0 := fillPixelBuffer(&pixel.Buffer{Layout: pixel.BufferRGB}, 0, defaultPixelsPerStrip)
		strip1 := fillPixelBuffer(&pixel.Buffer{Layout: pixel.BufferRGB}, 50, defaultPixelsPerStrip)
		strip2 := fillPixelBuffer(&pixel.Buffer{Layout: pixel.BufferRGB}, 100, defaultPixelsPerStrip)

		Context("when two packets fit", func() {
			BeforeEach(func() {
				ps.MaxStripsPerPacket = 2
			})

			It("will batch the first two packets, and send the third", func() {

				By("will buffer the first strip")
				err := ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 2, Pixels: *strip0})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(0))

				By("will send the first and second strips")
				err = ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 7, Pixels: *strip1})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(1))
				Expect(ds.datagrams[0]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCE},
					{2},
					strip0.Bytes(),
					{7},
					strip1.Bytes(),
				}, nil)))

				By("will buffer the third strip")
				err = ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 8, Pixels: *strip2})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(1))

				By("will send the third strip on flush")
				err = ps.Flush(ds)
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(2))
				Expect(ds.datagrams[1]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCF},
					{8},
					strip2.Bytes(),
				}, nil)))
			})
		})

		Context("when only one packet fits", func() {
			BeforeEach(func() {
				// [ID] + [StripNumber] + [StripBytes] + [Room...]
				ds.maxDatagramSize = 4 + 1 + len(strip0.Bytes()) + 10
			})

			It("will send each in their own packet", func() {
				By("will buffer the first strip")
				err := ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 2, Pixels: *strip0})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(0))

				By("will send the first strip and buffer the second")
				err = ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 6, Pixels: *strip1})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(1))
				Expect(ds.datagrams[0]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCE},
					{2},
					strip0.Bytes(),
				}, nil)))

				By("will send the second strip on flush")
				err = ps.Flush(ds)
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(2))
				Expect(ds.datagrams[1]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCF},
					{6},
					strip1.Bytes(),
				}, nil)))
			})
		})

		Context("when max strips per packet is 1", func() {
			BeforeEach(func() {
				ps.MaxStripsPerPacket = 1
			})

			It("will send each in their own packet immediately", func() {
				By("will send the first strip")
				err := ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 2, Pixels: *strip0})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(1))
				Expect(ds.datagrams[0]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCE},
					{2},
					strip0.Bytes(),
				}, nil)))

				By("will send the second")
				err = ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 6, Pixels: *strip1})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(2))
				Expect(ds.datagrams[1]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCF},
					{6},
					strip1.Bytes(),
				}, nil)))

				By("will not send any more on flush")
				err = ps.Flush(ds)
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(2))
			})
		})

		Context("with a fixed packet size", func() {
			BeforeEach(func() {
				// Two packets plus some room:
				// [ID] + 2*( [StripNumber] + [StripBytes] ) + [Room...]
				ps.FixedSize = 4 + 2*(1+len(strip0.Bytes())) + 10
				ps.MaxStripsPerPacket = 16
			})

			It("will buffer until the fixed size is full, then send", func() {
				By("will buffer the first strip")
				err := ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 2, Pixels: *strip0})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(0))

				By("will buffer the second strip")
				err = ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 4, Pixels: *strip1})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(0))

				By("will send the buffered strips, with padding, on third")
				err = ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 6, Pixels: *strip2})
				Expect(ds.datagrams).To(HaveLen(1))
				Expect(ds.datagrams[0]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCE},
					{2},
					strip0.Bytes(),
					{4},
					strip1.Bytes(),
					bytes.Repeat([]byte{0}, 10), // Padding
				}, nil)))
				Expect(len(ds.datagrams[0])).To(Equal(ps.FixedSize))

				By("will, on flush, send the final strip with padding")
				err = ps.Flush(ds)
				Expect(ds.datagrams).To(HaveLen(2))
				Expect(ds.datagrams[1]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCF},
					{6},
					strip2.Bytes(),
					bytes.Repeat([]byte{0}, 59), // Padding
				}, nil)))
				Expect(len(ds.datagrams[1])).To(Equal(ps.FixedSize))
			})
		})

		Context("when no packets fit", func() {
			BeforeEach(func() {
				ds.maxDatagramSize = 10 // Too small for any strips
			})

			It("will error on send", func() {
				err := ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 2, Pixels: *strip0})
				Expect(err).To(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(0))
			})
		})
	})

	Context("sending command and pixel packets", func() {

		Context("when a pixel packet is buffered", func() {
			strip0 := fillPixelBuffer(&pixel.Buffer{Layout: pixel.BufferRGB}, 0, defaultPixelsPerStrip)

			BeforeEach(func() {
				ds.maxDatagramSize = 1024
				ps.FixedSize = 0
				ps.MaxStripsPerPacket = 2

				err := ps.SendOrEnqueueStripState(ds, &StripState{StripNumber: 2, Pixels: *strip0})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(0))
			})

			It("will send the command immediately, then the buffered pixels", func() {
				err := ps.Send(ds, &Packet{Command: &ResetCommand{}})
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(1))
				Expect(ds.datagrams[0]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCE},
					CommandMagic,
					{byte(CommandReset)},
				}, nil)))

				By("flushing will send the strip")
				err = ps.Flush(ds)
				Expect(err).ToNot(HaveOccurred())
				Expect(ds.datagrams).To(HaveLen(2))
				Expect(ds.datagrams[1]).To(Equal(bytes.Join([][]byte{
					{0x00, 0x00, 0xFA, 0xCF},
					{2},
					strip0.Bytes(),
				}, nil)))
			})
		})
	})
})
