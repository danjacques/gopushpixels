// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"github.com/danjacques/gopushpixels/pixel"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/protocol/pixelpusher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mutable", func() {
	var m *Mutable
	BeforeEach(func() {
		m = &Mutable{}
	})

	Context("when empty", func() {
		It("will return false for SetPixel", func() {
			Expect(m.SetPixel(0, 0, pixel.P{})).To(BeFalse())
			Expect(m.SetPixel(100, 200, pixel.P{})).To(BeFalse())
			Expect(m.SetPixel(-5, -10, pixel.P{})).To(BeFalse())
		})

		It("will return zero-value pixels for GetPixel", func() {
			Expect(m.GetPixel(0, 0)).Should(BeZero())
			Expect(m.GetPixel(100, 200)).Should(BeZero())
			Expect(m.GetPixel(-5, -10)).Should(BeZero())
		})

		It("will return a nil SyncPacket", func() {
			Expect(m.SyncPacket()).To(BeNil())
		})
	})

	Context("with PixelPusher headers", func() {
		headers := protocol.DiscoveryHeaders{
			DeviceHeader: protocol.DeviceHeader{
				DeviceType: protocol.PixelPusherDeviceType,
			},
			PixelPusher: &pixelpusher.Device{
				DeviceHeader: pixelpusher.DeviceHeader{
					StripsAttached: 2,
					PixelsPerStrip: 4,
				},
				DeviceHeaderExt109: pixelpusher.DeviceHeaderExt109{
					StripFlags: []pixelpusher.StripFlags{
						// Strip 0 is RGB.
						0,
						// Strip 1 is RGBOW.
						pixelpusher.SFlagRGBOW,
					},
				},
			},
		}

		BeforeEach(func() {
			m.Initialize(&headers)
		})

		It("won't set a pixel on an invalid strip", func() {
			Expect(m.SetPixel(3, 0, pixel.P{})).To(BeFalse())
		})

		It("won't set a pixel beyond the end of a strip", func() {
			Expect(m.SetPixel(0, 5, pixel.P{})).To(BeFalse())
		})

		Context("after setting a pixel", func() {
			BeforeEach(func() {
				By("setting pixel in strip 0")
				set := m.SetPixel(0, 1, pixel.P{Red: 10, Green: 20, Blue: 30})
				Expect(set).To(BeTrue())

				By("setting pixel in strip 1")
				set = m.SetPixel(1, 3, pixel.P{Red: 50, Green: 100, Blue: 150, White: 200})
				Expect(set).To(BeTrue())
			})

			It("can get pixel values", func() {
				p := m.GetPixel(0, 1)
				Expect(p).To(Equal(pixel.P{Red: 10, Green: 20, Blue: 30}))

				p = m.GetPixel(1, 3)
				Expect(p).To(Equal(pixel.P{Red: 50, Green: 100, Blue: 150, White: 200}))
			})

			It("can generate a PixelPusher update packet", func() {
				pkt := m.SyncPacket()
				Expect(pkt).ToNot(BeNil())
				Expect(pkt.PixelPusher).ToNot(BeNil())

				pp := pkt.PixelPusher
				Expect(pp.Command).To(BeNil())
				Expect(pp.StripStates).To(HaveLen(2))

				var pixels pixel.Buffer

				By("verifying strip #0")
				pixels.Layout = pixel.BufferRGB
				pixels.SetPixels(
					pixel.P{},
					pixel.P{Red: 10, Green: 20, Blue: 30},
					pixel.P{},
					pixel.P{},
				)
				Expect(pp.StripStates[0].StripNumber).To(BeEquivalentTo(0))
				Expect(pp.StripStates[0].Pixels.Bytes()).To(BeEquivalentTo(pixels.Bytes()))

				By("verifying strip #1")
				pixels.Layout = pixel.BufferRGBOW
				pixels.SetPixels(
					pixel.P{},
					pixel.P{},
					pixel.P{},
					pixel.P{Red: 50, Green: 100, Blue: 150, White: 200},
				)
				Expect(pp.StripStates[1].StripNumber).To(BeEquivalentTo(1))
				Expect(pp.StripStates[1].Pixels.Bytes()).To(BeEquivalentTo(pixels.Bytes()))

				By("a second packet will be nil")
				Expect(m.SyncPacket()).To(BeNil())
			})
		})
	})
})
