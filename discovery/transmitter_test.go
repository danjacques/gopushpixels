// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package discovery

import (
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/network"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockDatagramSender struct {
	network.DatagramSender

	Datagrams [][]byte
}

func (mds *mockDatagramSender) SendDatagram(data []byte) error {
	mds.Datagrams = append(mds.Datagrams, append([]byte(nil), data...))
	return nil
}

var _ = Describe("Transmitter", func() {
	var (
		t   *Transmitter
		mds *mockDatagramSender
	)
	BeforeEach(func() {
		mds = &mockDatagramSender{}
		t = &Transmitter{}
	})

	It("will broadcast discovery headers", func() {
		dh := protocol.DiscoveryHeaders{}

		err := t.Broadcast(mds, &dh)
		Expect(err).ToNot(HaveOccurred())
		Expect(mds.Datagrams).To(HaveLen(1))
	})
})
