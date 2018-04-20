// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package network

import (
	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockDatagramSender struct {
	Packets [][]byte

	err    error
	closed bool
}

func (mds *mockDatagramSender) Close() error {
	mds.closed = true
	return mds.err
}

func (mds *mockDatagramSender) SendDatagram(b []byte) error {
	if mds.err != nil {
		return mds.err
	}

	mds.Packets = append(mds.Packets, append([]byte(nil), b...))
	return nil
}

func (mds *mockDatagramSender) MaxDatagramSize() int { return 0 }

var _ = Describe("ResilientDatagramSender", func() {
	var mocks []*mockDatagramSender
	var initialMockError error
	BeforeEach(func() {
		mocks, initialMockError = nil, nil
	})

	var factoryErr error
	var rds *ResilientDatagramSender
	BeforeEach(func() {
		rds = &ResilientDatagramSender{
			Factory: func() (DatagramSender, error) {
				if factoryErr != nil {
					return nil, factoryErr
				}
				mds := &mockDatagramSender{
					err: initialMockError,
				}
				mocks = append(mocks, mds)
				return mds, nil
			},
		}
	})

	It("will connect on initial SendDatagram", func() {
		By("the send should succeed")
		err := rds.SendDatagram([]byte("ohai"))
		Expect(err).ToNot(HaveOccurred())

		By("a second send should succeed")
		err = rds.SendDatagram([]byte("whatup"))
		Expect(err).ToNot(HaveOccurred())

		By("closing the connection")
		err = rds.Close()
		Expect(err).ToNot(HaveOccurred())

		By("it should have connected once")
		Expect(mocks).To(HaveLen(1))
		Expect(mocks[0].Packets).To(Equal([][]byte{
			[]byte("ohai"),
			[]byte("whatup"),
		}))
	})

	It("will reconnect after close", func() {
		By("connect")
		err := rds.Connect()
		Expect(err).ToNot(HaveOccurred())

		By("close")
		err = rds.Close()
		Expect(err).ToNot(HaveOccurred())

		Expect(mocks).To(HaveLen(1))
		Expect(mocks[0].closed).To(BeTrue())

		By("send opens new connection")
		err = rds.SendDatagram([]byte("yoyoyo"))
		Expect(err).ToNot(HaveOccurred())

		By("checking connections")
		Expect(mocks).To(HaveLen(2))
	})

	It("will reconnect if a SendDatagram fails", func() {
		By("first send (error)")
		initialMockError = errors.New("test error")
		err := rds.SendDatagram([]byte("ohai"))
		Expect(err).To(MatchError("test error"))

		By("a second send (success)")
		initialMockError = nil
		err = rds.SendDatagram([]byte("whatup"))
		Expect(err).ToNot(HaveOccurred())

		By("closing the connection")
		err = rds.Close()
		Expect(err).ToNot(HaveOccurred())

		By("checking connections")
		Expect(mocks).To(HaveLen(2))
		Expect(mocks[0].Packets).To(HaveLen(0))
		Expect(mocks[1].Packets).To(Equal([][]byte{
			[]byte("whatup"),
		}))
	})

	It("close forwards error", func() {
		By("connect")
		err := rds.Connect()
		Expect(err).ToNot(HaveOccurred())

		By("close (error)")
		Expect(mocks).To(HaveLen(1))
		mocks[0].err = errors.New("test error")

		err = rds.Close()
		Expect(err).To(MatchError("test error"))
		Expect(mocks[0].closed).To(BeTrue())

		By("send opens new connection")
		err = rds.SendDatagram([]byte("yoyoyo"))
		Expect(err).ToNot(HaveOccurred())

		By("checking connections")
		Expect(mocks).To(HaveLen(2))
		Expect(mocks[1].Packets).To(Equal([][]byte{
			[]byte("yoyoyo"),
		}))
	})
})
