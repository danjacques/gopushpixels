// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Strip Flags", func() {
	Context("an empty set of strip flags", func() {
		var sf StripFlags

		It("generates a string", func() {
			Expect(sf.String()).Should(Equal("0x00"))
		})

		It("does not have RGBOW set", func() {
			Expect(sf.IsRGBOW()).To(BeFalse())
		})

		Context("can set RGBOW", func() {
			sf := sf
			sf.SetRGBOW(true)

			It("generates a string", func() {
				Expect(sf.String()).Should(Equal("0x01(RGBOW)"))
			})

			It("has RGBOW set", func() {
				Expect(sf.IsRGBOW()).To(BeTrue())
			})
		})
	})

	Context("a full set of strip flags", func() {
		sf := StripFlags(SFlagRGBOW | SFlagWidePixels | SFlagLogarithmic | SFlagMotion |
			SFlagNotIdempotent | SFlagBrightness | SFlagMonochrome)

		It("generates a string", func() {
			Expect(sf.String()).Should(Equal(
				"0x7F(RGBOW|WIDEPIXELS|LOGARITHMIC|MOTION|NOTIDEMPOTENT|BRIGHTNESS|MONOCHROME)"))
		})

		Context("can unset RGBOW flag", func() {
			sf := sf
			sf.SetRGBOW(false)

			It("generates a string without RGBOW", func() {
				Expect(sf.String()).Should(Equal(
					"0x7E(WIDEPIXELS|LOGARITHMIC|MOTION|NOTIDEMPOTENT|BRIGHTNESS|MONOCHROME)"))
			})

			It("will not have RGBOW set", func() {
				Expect(sf.IsRGBOW()).To(BeFalse())
			})
		})
	})
})
