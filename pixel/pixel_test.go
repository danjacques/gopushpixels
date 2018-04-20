// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixel

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pixel String Conversion", func() {
	Context("an RGB pixel", func() {
		p := P{Red: 10, Green: 20, Blue: 30}
		It("generates a short string", func() {
			Expect(p.String()).Should(Equal("(10, 20, 30)"))
		})
	})

	Context("an RGBOW pixel", func() {
		p := P{Red: 50, Green: 100, Blue: 150, Orange: 200, White: 250}
		It("generates a long string", func() {
			Expect(p.String()).Should(Equal("(50, 100, 150 / 200, 250)"))
		})

		It("can do antilog conversion", func() {
			ap := p.AntiLog()

			Expect(ap).Should(Equal(P{
				Red:    2,
				Green:  8,
				Blue:   25,
				Orange: 76,
				White:  229,
			}))
		})
	})
})

func TestPixel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test pixel")
}
