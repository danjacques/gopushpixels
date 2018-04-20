// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/protocol/pixelpusher"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type testDevice struct {
	device.D

	id      string
	ordinal device.Ordinal
	headers protocol.DiscoveryHeaders
}

func makeTestDevice(id string, group, controller int) device.D {
	return &testDevice{
		id: id,
		headers: protocol.DiscoveryHeaders{
			PixelPusher: &pixelpusher.Device{
				DeviceHeader: pixelpusher.DeviceHeader{
					PixelsPerStrip: 1337,
				},
			},
		},
		ordinal: device.Ordinal{Group: group, Controller: controller},
	}
}

func (td *testDevice) ID() string                                   { return td.id }
func (td *testDevice) DiscoveryHeaders() *protocol.DiscoveryHeaders { return &td.headers }
func (td *testDevice) Ordinal() device.Ordinal                      { return td.ordinal }

var _ = Describe("End-to-End", func() {
	const readRounds = 5

	cwd, err := os.Getwd()
	if err != nil {
		panic(errors.Wrap(err, "could not get working directory"))
	}

	concat := func(b ...[]byte) []byte {
		return bytes.Join(b, []byte(nil))
	}

	mustTimestamp := func(v time.Time) *timestamp.Timestamp {
		ts, err := ptypes.TimestampProto(v)
		if err != nil {
			panic(err)
		}
		return ts
	}

	deviceFoo := makeTestDevice("foo", 2, 6)
	deviceBar := makeTestDevice("bar", 1, 5)

	instructions := []struct {
		device      device.D
		deviceIndex int64
		write       []byte
		duration    time.Duration

		expected *Event
	}{

		// Simple packet.
		{
			device:      deviceFoo,
			deviceIndex: 0,
			write:       []byte("ohai"),
			duration:    time.Second,
		},

		// Same ID, should be optimized out.
		{
			device:      deviceFoo,
			deviceIndex: 0,
			write:       []byte("whatup"),
			duration:    time.Second,
		},

		// Fully-optimized PixelPusher command packet.
		{
			device:      deviceBar,
			deviceIndex: 1,
			write:       concat(pixelpusher.CommandMagic, []byte("supdude")),
			duration:    time.Second,
		},
	}

	var tdir string
	BeforeEach(func() {
		var err error
		tdir, err = ioutil.TempDir(cwd, "basestream_test_data")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if tdir != "" {
			_ = os.RemoveAll(tdir)
			tdir = ""
		}
	})

	DescribeTable("different compression modes", func(comp Compression) {
		path := filepath.Join(tdir, "output.file")

		now := time.Now()
		cfg := EventStreamConfig{
			TempDir:           tdir,
			WriterCompression: comp,
			NowFunc:           func() time.Time { return now },
		}

		// Write our file.
		created := now
		sw, err := cfg.MakeEventStreamWriter(path, "File")
		Expect(err).ToNot(HaveOccurred())

		for _, insn := range instructions {
			// Make a fake pixel buffer.
			pkt := protocol.Packet{
				PixelPusher: &pixelpusher.Packet{
					StripStates: []*pixelpusher.StripState{
						{
							StripNumber: 5,
						},
					},
				},
			}
			pkt.PixelPusher.StripStates[0].Pixels.UseBytes(insn.write)

			err := sw.WritePacket(insn.device, &pkt)
			Expect(err).ToNot(HaveOccurred())

			// Advance "time".
			now = now.Add(insn.duration)
		}

		err = sw.Close()
		Expect(err).ToNot(HaveOccurred())

		// Read that file (multiple times).
		sr, err := MakeEventStreamReader(path)
		Expect(err).ToNot(HaveOccurred())

		for i := 0; i < readRounds; i++ {
			expected := &Metadata{
				Version:   Metadata_V_1,
				Minor:     1,
				Name:      "File",
				NumEvents: int64(len(instructions)),
				EventFileInfo: []*Metadata_EventFile{
					{
						Name:        "events.protostream",
						Compression: comp,

						// (foo, bar), which is the order that the devices were recognized.
						DeviceMapping: []int64{1, 0},
						Duration:      ptypes.DurationProto(2 * time.Second),
						NumEvents:     3,
					},
				},
				Created:  mustTimestamp(created),
				Duration: ptypes.DurationProto(2 * time.Second),
				Devices: []*Device{
					{
						Id:             "bar",
						PixelsPerStrip: 1337,
						Ordinal: &Device_Ordinal{
							Group:      1,
							Controller: 5,
						},
					},
					{
						Id:             "foo",
						PixelsPerStrip: 1337,
						Ordinal: &Device_Ordinal{
							Group:      2,
							Controller: 6,
						},
					},
				},
			}

			// Rather than hard-code the number of bytes this will take, we just
			// assert that we didn't record 0.
			//
			// Since we modify it, we will clone it first.
			md := proto.Clone(sr.Metadata()).(*Metadata)
			Expect(md.NumBytes).ToNot(Equal(0))
			md.NumBytes = 0

			for _, efi := range md.EventFileInfo {
				Expect(efi.NumBytes).ToNot(Equal(0))
				efi.NumBytes = 0
			}

			Expect(proto.Equal(md, expected)).To(BeTrue(),
				"invalid metadata:\nExpected: %s\nActual:   %s", expected, md)

			for i, insn := range instructions {
				// Read/validate first event.
				e, err := sr.ReadEvent()
				Expect(err).ToNot(HaveOccurred())

				if insn.expected == nil {
					// Use "write" as the expected packet data.
					pkt := e.GetPacket()
					Expect(pkt).ToNot(BeNil(), "packet #%d is nil", i)
					Expect(pkt.Device).To(Equal(insn.deviceIndex))

					pixels := pkt.GetPixelpusherPixels()
					Expect(pixels).ToNot(BeNil())
					Expect(proto.Equal(pixels, &PixelPusherPixels{
						StripNumber: 5,
						PixelData:   insn.write,
					})).To(BeTrue(), "data does not match: %v", pixels.PixelData)
				} else {
					// Compare events directly.
					Expect(proto.Equal(e, insn.expected)).To(BeTrue(),
						"event proto does not match:\nExpected: %s\nActual: %s", insn.expected, e)
				}
			}

			// One more read will yield EOF.
			_, err := sr.ReadEvent()
			Expect(err).To(Equal(io.EOF))

			// Reset for next round.
			err = sr.Reset()
			Expect(err).ToNot(HaveOccurred())
		}

		err = sr.Close()
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("SNAPPY", Compression_NONE),
		Entry("SNAPPY", Compression_SNAPPY),
		Entry("SNAPPY", Compression_GZIP),
	)
})

func TestStreamFile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testing streamfile")
}
