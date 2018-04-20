// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command Parsing", func() {
	var entries = []TableEntry{
		Entry("Reset", []byte{0x01}, &ResetCommand{}),

		Entry("GlobalBrightnessSet",
			[]byte{
				0x02,
				0x34, 0x12,
			},
			&GlobalBrightnessSetCommand{
				Parameter: 0x1234,
			}),

		Entry("StripBrightnessSet",
			[]byte{
				0x05,
				0x7F,
				0x34, 0x12,
			},
			&StripBrightnessSetCommand{
				StripNumber: 0x7F,
				Parameter:   0x1234,
			}),

		Entry("WiFiConfigureCommand",
			bytes.Join([][]byte{
				{0x03},
				[]byte("ohai there\x00"),
				[]byte("I'm a key!\x00"),
				{0x02}, // WPA
			}, nil),
			&WiFiConfigureCommand{
				SSID:     "ohai there",
				Key:      "I'm a key!",
				Security: SecurityWPA,
			}),

		Entry("LEDConfigureCommand",
			[]byte{
				0x04,
				0x78, 0x56, 0x34, 0x12,
				0xEF, 0xCD, 0xAB, 0x89,
				0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01, 0x00,
				0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // BGR
				0xCE, 0xFA,
				0xEF, 0xBE,
				0x55, 0x44,
				0x33, 0x22,
			},
			&LEDConfigureCommand{
				NumStrips:      0x12345678,
				StripLength:    0x89ABCDEF,
				StripType:      0x0001020304050607,
				ColourOrder:    ColourOrderBGR,
				Group:          0xFACE,
				Controller:     0xBEEF,
				ArtNetUniverse: 0x4455,
				ArtNetChannel:  0x2233,
			}),
	}

	DescribeTable("command data (without magic)",
		func(data []byte, expected Command) {
			r := bytes.NewReader(data)

			By("decoding command")
			decoded, err := ReadCommand(r, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(decoded).To(Equal(expected))

			By("re-encoding command")
			var buf bytes.Buffer
			err = WriteCommand(decoded, &buf, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.Bytes()).To(BeEquivalentTo(data))
		}, entries...)

	DescribeTable("command data (with magic)",
		func(data []byte, expected Command) {
			// Prepend CommandMagic.
			data = bytes.Join([][]byte{CommandMagic, data}, nil)
			r := bytes.NewReader(data)

			By("decoding command")
			decoded, err := ReadCommand(r, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(decoded).To(Equal(expected))

			By("re-encoding command")
			var buf bytes.Buffer
			err = WriteCommand(decoded, &buf, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.Bytes()).To(BeEquivalentTo(data))
		}, entries...)
})
