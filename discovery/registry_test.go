// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package discovery

import (
	"net"
	"time"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Registry", func() {
	// expirationThreshold is the expiration threshold to use on registry entries.
	//
	// It should be large enough that trivial test methods don't run the risk of
	// hitting it on any testing systems, but small enough that it doesn't cause
	// our tests to take too long.
	//
	// If flake is observed, this can be increased.
	const (
		expirationThreshold = 500 * time.Millisecond

		observeThreshold = expirationThreshold / 10
	)
	timeoutThreshold := (expirationThreshold * 3).Seconds()

	var reg *Registry
	BeforeEach(func() {
		reg = &Registry{
			Expiration: expirationThreshold,
		}
	})

	AfterEach(func() {
		if reg != nil {
			reg.Shutdown()
		}
	})

	h0 := protocol.DiscoveryHeaders{
		DeviceHeader: protocol.DeviceHeader{
			MacAddress: [6]byte{0, 0, 0, 0, 0, 0},
			ProductID:  0,
		},
	}
	h1 := protocol.DiscoveryHeaders{
		DeviceHeader: protocol.DeviceHeader{
			MacAddress: [6]byte{0, 0, 0, 0, 0, 1},
			ProductID:  0,
		},
	}

	It("returns an empty list of devices by default", func() {
		devices := reg.Devices()
		Expect(devices).To(BeEmpty())
	})

	It("will do nothing if an unregistered device is Unregistered", func() {
		stub := device.MakeRemoteStub("stub", &net.UDPAddr{})
		reg.Unregister(stub)
	})

	Context("when two devices are newly observed", func() {
		var d0, d1 device.D

		BeforeEach(func() {
			var isNew bool
			d0, isNew = reg.Observe(&h0)
			Expect(isNew).To(BeTrue())
			Expect(d0).ToNot(BeNil())

			d1, isNew = reg.Observe(&h1)
			Expect(isNew).To(BeTrue())
			Expect(d1).ToNot(BeNil())
		})

		It("should list them in its Devices", func() {
			devices := reg.Devices()
			Expect(devices).To(ConsistOf(d0, d1))
		})

		Context("and they expire after being unobserved", func() {
			BeforeEach(func(done Done) {
				defer close(done)
				<-d0.DoneC()
				<-d1.DoneC()
			}, timeoutThreshold)

			It("should no longer list the devices", func() {
				devices := reg.Devices()
				Expect(devices).To(BeEmpty())
			})

			It("should have closed the devices", func() {
				d0Done := device.IsDone(d0)
				d1Done := device.IsDone(d1)

				Expect(d0Done).To(BeTrue())
				Expect(d1Done).To(BeTrue())
			})
		})

		Context("and explicitly unregisters d0", func() {
			BeforeEach(func() {
				reg.Unregister(d0)
			})

			It("should no longer list the device", func() {
				devices := reg.Devices()
				Expect(devices).To(ConsistOf(d1))
			})

			It("should have closed d0, but not d1", func() {
				d0Done := device.IsDone(d0)
				d1Done := device.IsDone(d1)

				Expect(d0Done).To(BeTrue())
				Expect(d1Done).To(BeFalse())
			})
		})

		Context("and d0 is repeatedly observed", func() {
			// Loop, repeatedly observing d0, until d1 expires.
			BeforeEach(func(done Done) {
				defer close(done)

				timer := time.NewTimer(observeThreshold)
				defer timer.Stop()

				for {
					select {
					case <-d1.DoneC():
						return
					case <-timer.C:
						d, isNew := reg.Observe(&h0)
						Expect(d).To(Equal(d0))
						Expect(isNew).To(BeFalse())

						// Reset the timer. This is safe, since we know that it triggered.
						timer.Reset(observeThreshold)
					}
				}
			}, timeoutThreshold)

			It("will only list d0 in devices", func() {
				devices := reg.Devices()
				Expect(devices).To(ConsistOf(d0))
			})

			It("can re-observe d1 as a different device", func() {
				By("observe the new device")
				dN, isNew := reg.Observe(&h1)
				Expect(isNew).To(BeTrue())
				Expect(dN).ToNot(Equal(d1))

				By("and list it in devices")
				devices := reg.Devices()
				Expect(devices).To(ConsistOf(d0, dN))
			})
		})
	})

	Context("when a device is repeatedly observed with new headers", func() {
		var d0 device.D

		// Observe repeatedly, incrementing product ID to mark a different header
		// set.
		BeforeEach(func() {
			// Initial observation.
			var isNew bool
			d0, isNew = reg.Observe(&h0)
			Expect(d0).ToNot(BeNil())
			Expect(isNew).To(BeTrue())

			for i := uint16(1); i <= 10; i++ {
				headers := h0
				headers.ProductID = i

				d0, isNew = reg.Observe(&headers)
				Expect(d0).ToNot(BeNil())
				Expect(isNew).To(BeFalse())

				time.Sleep(observeThreshold)
			}
		})

		It("should have the latest header values for d0", func() {
			dh := d0.DiscoveryHeaders()
			Expect(dh.ProductID).To(BeEquivalentTo(10))
		})
	})
})
