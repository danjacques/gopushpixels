// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ordinal", func() {
	Context("an invalid ordinal", func() {
		o := InvalidOrdinal()

		It("can be rendered to string", func() {
			Expect(o.String()).To(Equal("{INVALID}"))
		})

		It("will recognize itself as invalid", func() {
			Expect(o.IsValid()).To(BeFalse())
		})
	})

	Context("an valid ordinal", func() {
		o := Ordinal{
			Group:      0,
			Controller: 0,
		}

		It("can be rendered to string", func() {
			Expect(o.String()).To(Equal("{Grp=0, Cont=0}"))
		})

		It("will recognize itself as valid", func() {
			Expect(o.IsValid()).To(BeTrue())
		})
	})
})
