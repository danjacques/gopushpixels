// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package protostream

import (
	"bytes"
	"testing"
	"time"

	"github.com/danjacques/gopushpixels/support/dataio"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/struct"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("End-to-End Encode/Decode", func() {
	It("Can encode and then decode protobufs", func() {
		var buf bytes.Buffer

		p0 := structpb.Struct{
			Fields: map[string]*structpb.Value{
				"foo": {Kind: &structpb.Value_ListValue{
					ListValue: &structpb.ListValue{
						Values: []*structpb.Value{
							{Kind: &structpb.Value_StringValue{StringValue: "foo"}},
							{Kind: &structpb.Value_StringValue{StringValue: "bar"}},
						},
					},
				},
				},
			},
		}
		p1 := empty.Empty{}
		p2 := ptypes.DurationProto(time.Minute)

		// Encode some protobufs to "buf".
		var enc Encoder
		mustEncode := func(pb proto.Message, expectedSize int) {
			amt, err := enc.Write(&buf, pb)
			Expect(err).ToNot(HaveOccurred(), "while encoding %s", pb)
			Expect(amt).To(Equal(expectedSize), "while encoding %s", pb)
		}

		mustEncode(&p0, 26)
		mustEncode(&p1, 1)
		mustEncode(p2, 3)

		// Decode them back into containers.
		var dec Decoder
		dr := dataio.MakeReader(bytes.NewReader(buf.Bytes()))
		mustDecode := func(pb proto.Message, expectedMessage proto.Message, expectedSize int64) {
			amt, err := dec.Read(dr, pb)
			Expect(err).ToNot(HaveOccurred(), "while decoding %s", pb)
			Expect(amt).To(Equal(expectedSize), "while decoding %s", pb)
			Expect(proto.Equal(pb, expectedMessage)).To(BeTrue(),
				"messages are not equal (%v != %v)", pb, expectedMessage)
		}
		mustDecode(&structpb.Struct{}, &p0, 26)
		mustDecode(&empty.Empty{}, &p1, 1)
		mustDecode(&duration.Duration{}, p2, 3)
	})
})

func TestProtoStream(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testing protostream")
}
