// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registry", func() {
	var reg *Registry
	BeforeEach(func() {
		reg = &Registry{}
	})

	Context("with a registered device", func() {
		var d0 *testD
		BeforeEach(func() {
			d0 = makeTestD("foo")
			d0.ordinal = Ordinal{Group: 5, Controller: 10}

			reg.Add(d0)
		})
		AfterEach(func() {
			d0.markDone()
		})

		It("is registered", func() {
			d := reg.Get("foo")
			Expect(d).To(Equal(d0))

			By("unregisters when done")
			d0.markDone()
			d = reg.Get("foo")
			Expect(d).To(BeNil())
		})

		It("is registered under its unique ordinal", func() {
			d := reg.GetUniqueOrdinal(d0.ordinal)
			Expect(d).To(Equal(d0))

			By("unregisters when done")
			d0.markDone()
			d = reg.GetUniqueOrdinal(d0.ordinal)
			Expect(d).To(BeNil())
		})

		It("is registered under its group", func() {
			devices := reg.DevicesForGroup(5)
			Expect(devices).To(ContainElement(d0))

			By("unregisters when done")
			d0.markDone()
			devices = reg.DevicesForGroup(5)
			Expect(devices).To(BeEmpty())
		})

		Context("when re-registered with a different ordinal", func() {
			BeforeEach(func() {
				d0.ordinal = Ordinal{Group: 6, Controller: 15}
				reg.Add(d0)
			})

			It("is registered under its unique ordinal", func() {
				d := reg.GetUniqueOrdinal(d0.ordinal)
				Expect(d).To(Equal(d0))

				By("unregisters when done")
				d0.markDone()
				d = reg.GetUniqueOrdinal(d0.ordinal)
				Expect(d).To(BeNil())
			})

			It("is no longer registered under its old ordinal", func() {
				d := reg.GetUniqueOrdinal(Ordinal{Group: 5, Controller: 10})
				Expect(d).To(BeNil())
			})
		})

		Context("when a second device shares its Ordinal", func() {
			var d1 *testD
			BeforeEach(func() {
				d1 = makeTestD("bar")
				d1.ordinal = d0.ordinal
				reg.Add(d1)
			})
			AfterEach(func() {
				d1.markDone()
			})

			It("no longer is returned for its unique ordinal", func() {
				d := reg.GetUniqueOrdinal(d0.ordinal)
				Expect(d).To(BeNil())
			})
		})
	})

	Context("with multiple devices in multiple groups", func() {
		const count = 10

		var ordinalMap [][]*testD
		BeforeEach(func() {
			ordinalMap = make([][]*testD, count)

			for i := 0; i < count; i++ {
				for j := 0; j < count; j++ {
					d := makeTestD(fmt.Sprintf("%d.%d", i, j))
					d.ordinal = Ordinal{Group: i, Controller: j}

					reg.Add(d)
					ordinalMap[i] = append(ordinalMap[i], d)
				}
			}
		})

		It("has an entry for each device in its group map", func() {
			gmap := reg.AllGroups()
			for i := 0; i < count; i++ {
				devices := gmap[i]
				Expect(devices).To(HaveLen(count), "failed on group %d", i)
				Expect(devices).To(ConsistOf(ordinalMap[i]))
			}
		})

		It("lists devices for a given group", func() {
			for i := 0; i < count; i++ {
				devices := reg.DevicesForGroup(i)
				Expect(devices).To(HaveLen(count), "failed on group %d", i)
				Expect(devices).To(ConsistOf(ordinalMap[i]))
			}
		})

		Context("and the devices are closed", func() {
			BeforeEach(func() {
				for _, devices := range ordinalMap {
					for _, d := range devices {
						d.markDone()
					}
				}
			})

			It("clears the map when devices are closed", func() {
				gmap := reg.AllGroups()
				Expect(gmap).To(BeEmpty())
			})

			It("clears each group list", func() {
				for i := 0; i < count; i++ {
					devices := reg.DevicesForGroup(i)
					Expect(devices).To(BeEmpty())
				}
			})
		})
	})
})
