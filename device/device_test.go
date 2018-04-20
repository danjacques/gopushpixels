// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"testing"

	"github.com/danjacques/gopushpixels/protocol"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testD struct {
	D

	id      string
	ordinal Ordinal
	headers protocol.DiscoveryHeaders

	datagrams [][]byte
	packets   []*protocol.Packet

	doneC chan struct{}
	done  bool
}

func makeTestD(id string) *testD {
	return &testD{
		id:    id,
		doneC: make(chan struct{}),
	}
}

func (td *testD) ID() string       { return td.id }
func (td *testD) Ordinal() Ordinal { return td.ordinal }
func (td *testD) DiscoveryHeaders() *protocol.DiscoveryHeaders {
	return &td.headers
}
func (td *testD) DoneC() <-chan struct{} { return td.doneC }

func (td *testD) Sender() (Sender, error) {
	return &testSender{d: td}, nil
}

func (td *testD) markDone() {
	if !td.done {
		close(td.doneC)
		td.done = true
	}
}

type testSender struct {
	Sender
	d *testD
}

func (ts *testSender) SendDatagram(v []byte) error {
	ts.d.datagrams = append(ts.d.datagrams, append([]byte(nil), v...))
	return nil
}

func (ts *testSender) SendPacket(pkt *protocol.Packet) error {
	ts.d.packets = append(ts.d.packets, pkt)
	return nil
}

func (ts *testSender) Close() error { return nil }

var _ = Describe("IsDone", func() {
	var td *testD
	BeforeEach(func() {
		td = makeTestD("foo")
	})

	It("returns false if the device is not done.", func() {
		Expect(IsDone(td)).To(BeFalse())
	})

	It("returns true if the device is done.", func() {
		close(td.doneC)
		Expect(IsDone(td)).To(BeTrue())
	})
})

func TestDevice(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Device Tests")
}
