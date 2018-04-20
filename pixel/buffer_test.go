// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixel

import (
	"github.com/danjacques/gopushpixels/support/byteslicereader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pixel Buffer", func() {
	// Two RGB pixels: (100, 110, 120), (150, 160, 170)
	rawRGB := []byte{100, 110, 120, 150, 160, 170}

	// Two RGBOW pixels: (100, 110, 120, 130, 140), (150, 160, 170, 180, 190)
	rawRGBOW := []byte{
		100, 110, 120, 130, 130, 130, 140, 140, 140,
		150, 160, 170, 180, 180, 180, 190, 190, 190}

	Context("an RGB Buffer", func() {
		var pb *Buffer
		BeforeEach(func() {
			pb = &Buffer{Layout: BufferRGB}
		})

		It("has length 0", func() {
			Expect(pb.Len()).To(Equal(0))
			Expect(pb.Bytes()).To(HaveLen(0))
		})

		It("will grow its buffer when reset", func() {
			pb.Reset(5)
			Expect(pb.Bytes()).To(HaveLen(15))
		})

		It("can load the data using UseBytes", func() {
			pb.UseBytes(rawRGB)

			Expect(pb.Len()).To(Equal(2))
			Expect(pb.Pixel(0)).To(Equal(P{Red: 100, Green: 110, Blue: 120}))
			Expect(pb.Pixel(1)).To(Equal(P{Red: 150, Green: 160, Blue: 170}))
			Expect(pb.Bytes()).To(HaveLen(6))
		})

		Context("when loading the data using ReadFrom", func() {

			Context("with loaded data", func() {
				BeforeEach(func() {
					// NOTE: We copy b/c AntiLog does a buffer transformation.
					err := pb.ReadFrom(&byteslicereader.R{Buffer: rawRGB, AlwaysCopy: true}, 2)
					Expect(err).ToNot(HaveOccurred())
				})

				It("has the correct pixels", func() {
					Expect(pb.Len()).To(Equal(2))
					Expect(pb.Pixel(0)).To(Equal(P{Red: 100, Green: 110, Blue: 120}))
					Expect(pb.Pixel(1)).To(Equal(P{Red: 150, Green: 160, Blue: 170}))
					Expect(pb.Bytes()).To(HaveLen(6))
				})

				It("can be transformed via AntiLog", func() {
					pb.AntiLog()

					Expect(pb.Pixel(0)).To(Equal(P{Red: 8, Green: 10, Blue: 13}))
					Expect(pb.Pixel(1)).To(Equal(P{Red: 25, Green: 31, Blue: 39}))
				})
			})

			It("will error if the buffer is incomplete", func() {
				raw := []byte{1, 2, 3, 4, 5}
				err := pb.ReadFrom(&byteslicereader.R{Buffer: raw}, 2)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when cloning", func() {
			It("can clone a like buffer", func() {
				other := Buffer{Layout: BufferRGB}
				other.UseBytes(rawRGB)

				pb.CloneFrom(&other)
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGB))

				// Mutating pb should not change other.
				pb.AntiLog()
				Expect(pb.Bytes()).ToNot(BeEquivalentTo(rawRGB))
				Expect(other.Bytes()).To(BeEquivalentTo(rawRGB))
			})

			It("can clone a like buffer with length", func() {
				other := Buffer{Layout: BufferRGB}
				other.UseBytes(rawRGB)

				pb.CloneFromWithLen(&other, 1)
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGB[:3]))

				// Mutating pb should not change other.
				pb.AntiLog()
				Expect(pb.Bytes()).ToNot(BeEquivalentTo(rawRGB[:3]))
				Expect(other.Bytes()).To(BeEquivalentTo(rawRGB))
			})

			It("can clone an RGBOW buffer", func() {
				other := Buffer{Layout: BufferRGBOW}
				other.UseBytes(rawRGBOW)

				pb.CloneFrom(&other)
				Expect(pb.Layout).To(Equal(BufferRGBOW))
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGBOW))
			})
		})

		Context("when using CopyPixelValues", func() {
			It("can copy a like buffer", func() {
				other := Buffer{Layout: BufferRGB}
				other.UseBytes(rawRGB)

				pb.Reset(other.Len())
				pb.CopyPixelValuesFrom(&other)
				Expect(pb.Layout).To(Equal(BufferRGB))
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGB))
			})

			It("can copy one pixel from a like buffer", func() {
				other := Buffer{Layout: BufferRGB}
				other.UseBytes(rawRGB)

				pb.Reset(1)
				pb.CopyPixelValuesFrom(&other)
				Expect(pb.Layout).To(Equal(BufferRGB))
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGB[:3]))
			})

			It("can copy an RGBOW buffer", func() {
				other := Buffer{Layout: BufferRGBOW}
				other.UseBytes(rawRGBOW)

				pb.Reset(other.Len())
				pb.CopyPixelValuesFrom(&other)
				Expect(pb.Layout).To(Equal(BufferRGB))
				Expect(pb.Pixel(0)).To(Equal(P{Red: 100, Green: 110, Blue: 120}))
				Expect(pb.Pixel(1)).To(Equal(P{Red: 150, Green: 160, Blue: 170}))
			})

			It("can set pixels", func() {
				pb.SetPixels(P{Red: 10}, P{Green: 20, Orange: 100}, P{Blue: 30, White: 200})

				Expect(pb.Len()).To(Equal(3))
				Expect(pb.Pixel(0)).To(Equal(P{Red: 10}))
				Expect(pb.Pixel(1)).To(Equal(P{Green: 20}))
				Expect(pb.Pixel(2)).To(Equal(P{Blue: 30}))
			})
		})

		Context("manipulating pixels", func() {
			It("will ignore out-of-bounds pixels", func() {
				pb.SetPixel(1337, P{})
			})

			It("can mutate pixels", func() {
				pb.Reset(2)

				By("setting pixels")
				pb.SetPixel(0, P{Red: 1, Green: 2, Blue: 3, Orange: 4, White: 5})
				pb.SetPixel(1, P{Red: 6, Green: 7, Blue: 8, Orange: 9, White: 10})

				By("reading pixels")
				Expect(pb.Pixel(0)).To(Equal(P{Red: 1, Green: 2, Blue: 3}))
				Expect(pb.Pixel(1)).To(Equal(P{Red: 6, Green: 7, Blue: 8}))

				By("reading pixel buffer")
				Expect(pb.Bytes()).To(BeEquivalentTo([]byte{1, 2, 3, 6, 7, 8}))
			})
		})
	})

	Context("an RGBOW Buffer", func() {
		var pb *Buffer
		BeforeEach(func() {
			pb = &Buffer{Layout: BufferRGBOW}
		})

		It("has length 0", func() {
			Expect(pb.Len()).To(Equal(0))
		})

		It("will grow its buffer when reset", func() {
			pb.Reset(5)
			Expect(pb.Bytes()).To(HaveLen(45))
		})

		It("can load the data using UseBytes", func() {
			pb.UseBytes(rawRGBOW)

			Expect(pb.Len()).To(Equal(2))
			Expect(pb.Pixel(0)).To(Equal(P{Red: 100, Green: 110, Blue: 120, Orange: 130, White: 140}))
			Expect(pb.Pixel(1)).To(Equal(P{Red: 150, Green: 160, Blue: 170, Orange: 180, White: 190}))
			Expect(pb.Bytes()).To(HaveLen(18))
		})

		Context("when loading the data using ReadFrom", func() {

			Context("with loaded data", func() {
				BeforeEach(func() {
					// NOTE: We copy b/c AntiLog does a buffer transformation.
					err := pb.ReadFrom(&byteslicereader.R{Buffer: rawRGBOW, AlwaysCopy: true}, 2)
					Expect(err).ToNot(HaveOccurred())
				})

				It("has the correct pixels", func() {
					Expect(pb.Len()).To(Equal(2))
					Expect(pb.Pixel(0)).To(Equal(P{Red: 100, Green: 110, Blue: 120, Orange: 130, White: 140}))
					Expect(pb.Pixel(1)).To(Equal(P{Red: 150, Green: 160, Blue: 170, Orange: 180, White: 190}))
					Expect(pb.Bytes()).To(HaveLen(18))
				})

				It("can be transformed via AntiLog", func() {
					pb.AntiLog()

					Expect(pb.Pixel(0)).To(Equal(P{Red: 8, Green: 10, Blue: 13, Orange: 16, White: 20}))
					Expect(pb.Pixel(1)).To(Equal(P{Red: 25, Green: 31, Blue: 39, Orange: 49, White: 61}))
				})
			})

			It("will error if the buffer is incomplete", func() {
				raw := []byte{1, 2, 3, 4, 4, 4, 5, 5, 5, 6, 7}
				err := pb.ReadFrom(&byteslicereader.R{Buffer: raw}, 2)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when cloning", func() {
			It("can clone a like buffer", func() {
				other := Buffer{Layout: BufferRGBOW}
				other.UseBytes(rawRGBOW)

				pb.CloneFrom(&other)
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGBOW))

				// Mutating pb should not change other.
				pb.AntiLog()
				Expect(pb.Bytes()).ToNot(BeEquivalentTo(rawRGBOW))
				Expect(other.Bytes()).To(BeEquivalentTo(rawRGBOW))
			})

			It("can clone a like buffer with length", func() {
				other := Buffer{Layout: BufferRGBOW}
				other.UseBytes(rawRGBOW)

				pb.CloneFromWithLen(&other, 1)
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGBOW[:9]))

				// Mutating pb should not change other.
				pb.AntiLog()
				Expect(pb.Bytes()).ToNot(BeEquivalentTo(rawRGBOW[:9]))
				Expect(other.Bytes()).To(BeEquivalentTo(rawRGBOW))
			})

			It("can clone an RGB buffer", func() {
				other := Buffer{Layout: BufferRGB}
				other.UseBytes(rawRGB)

				pb.CloneFrom(&other)
				Expect(pb.Layout).To(Equal(BufferRGB))
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGB))
			})
		})

		Context("when using CopyPixelValues", func() {
			It("can copy a like buffer", func() {
				other := Buffer{Layout: BufferRGBOW}
				other.UseBytes(rawRGBOW)

				pb.Reset(other.Len())
				pb.CopyPixelValuesFrom(&other)
				Expect(pb.Layout).To(Equal(BufferRGBOW))
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGBOW))
			})

			It("can copy one pixel from a like buffer", func() {
				other := Buffer{Layout: BufferRGBOW}
				other.UseBytes(rawRGBOW)

				pb.Reset(1)
				pb.CopyPixelValuesFrom(&other)
				Expect(pb.Layout).To(Equal(BufferRGBOW))
				Expect(pb.Bytes()).To(BeEquivalentTo(rawRGBOW[:9]))
			})

			It("can copy an RGB buffer", func() {
				other := Buffer{Layout: BufferRGB}
				other.UseBytes(rawRGB)

				pb.Reset(other.Len())
				pb.CopyPixelValuesFrom(&other)
				Expect(pb.Layout).To(Equal(BufferRGBOW))
				Expect(pb.Pixel(0)).To(Equal(P{Red: 100, Green: 110, Blue: 120}))
				Expect(pb.Pixel(1)).To(Equal(P{Red: 150, Green: 160, Blue: 170}))
			})

			It("can set pixels", func() {
				pb.SetPixels(P{Red: 10}, P{Green: 20, Orange: 100}, P{Blue: 30, White: 200})

				Expect(pb.Len()).To(Equal(3))
				Expect(pb.Pixel(0)).To(Equal(P{Red: 10}))
				Expect(pb.Pixel(1)).To(Equal(P{Green: 20, Orange: 100}))
				Expect(pb.Pixel(2)).To(Equal(P{Blue: 30, White: 200}))
			})
		})

		Context("manipulating pixels", func() {
			It("will ignore out-of-bounds pixels", func() {
				pb.SetPixel(1337, P{})
			})

			It("can mutate pixels", func() {
				pb.Reset(2)

				By("setting pixels")
				pb.SetPixel(0, P{Red: 1, Green: 2, Blue: 3, Orange: 4, White: 5})
				pb.SetPixel(1, P{Red: 6, Green: 7, Blue: 8, Orange: 9, White: 10})

				By("reading pixels")
				Expect(pb.Pixel(0)).To(Equal(P{Red: 1, Green: 2, Blue: 3, Orange: 4, White: 5}))
				Expect(pb.Pixel(1)).To(Equal(P{Red: 6, Green: 7, Blue: 8, Orange: 9, White: 10}))

				By("reading pixel buffer")
				Expect(pb.Bytes()).To(BeEquivalentTo([]byte{
					1, 2, 3, 4, 4, 4, 5, 5, 5,
					6, 7, 8, 9, 9, 9, 10, 10, 10}))
			})
		})
	})
})
