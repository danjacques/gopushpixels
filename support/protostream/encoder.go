package protostream

import (
	"io"

	"github.com/golang/protobuf/proto"
)

// Encoder encodes a protobuf packet stream to an io.Writer.
type Encoder struct {
	buf *proto.Buffer
}

func (e *Encoder) Write(w io.Writer, pb proto.Message) (int, error) {
	if e.buf == nil {
		e.buf = proto.NewBuffer(nil)
	} else {
		e.buf.Reset()
	}

	// Encode the size prefix, as a varint.
	if err := e.buf.EncodeVarint(uint64(proto.Size(pb))); err != nil {
		return 0, err
	}

	// Encode the protobuf message.
	if err := e.buf.Marshal(pb); err != nil {
		return 0, err
	}

	// Write the full buffer to "w".
	return w.Write(e.buf.Bytes())
}
