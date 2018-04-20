package protostream

import (
	"bytes"
	"errors"
	"io"

	"github.com/danjacques/gopushpixels/support/dataio"

	"github.com/golang/protobuf/proto"
)

// The maximum varint size, in bytes. This is the total number of bytes needed
// to encode the largest uint64 using proto.EncodeVarint.
const maxVarintSizeU64 = 10

// Decoder is a reusable object which decodes a series of messages from a proto
// stream.
type Decoder struct {
	buf     *proto.Buffer
	dataBuf bytes.Buffer

	sizeBuf [maxVarintSizeU64]byte
}

func (d *Decoder) bufferNextVarint(r dataio.Reader) ([]byte, error) {
	sizeBuf := d.sizeBuf[:0]
	for len(sizeBuf) < maxVarintSizeU64 {
		// Allocate room for an additional byte.
		b, err := r.ReadByte()
		if err != nil {
			return sizeBuf, err
		}

		sizeBuf = append(sizeBuf, b)
		if (b & 0x80) == 0 {
			// Varint does not have continuation bit set.
			return sizeBuf, nil
		}
	}

	// If we've reached our maximum size, error.
	return sizeBuf, errors.New("size prefix is not a valid varint")
}

// Read reads data byte-by-byte. Users should use a buffered Reader.
func (d *Decoder) Read(r dataio.Reader, pb proto.Message) (int64, error) {
	// Initialization (first time).
	if d.buf == nil {
		d.buf = proto.NewBuffer(nil)
	}

	// Read the next varint. To do this, we read byte-by-byte until we find a
	// non-terminated varint buffer.
	//
	// The "proto" package doesn't help with this; instead, we use an
	// implementation detail: the varint continues until the most significant
	// bit is zero.
	sizeBuf, err := d.bufferNextVarint(r)
	count := int64(len(sizeBuf))
	if err != nil {
		return count, err
	}

	// sizeBuf contains the full varint. Decode it using the Buffer. Since we've
	// vetted the value in the previous loop, this must work.
	size, amt := proto.DecodeVarint(sizeBuf)
	if amt != len(sizeBuf) {
		panic("incompatible proto varint encoding")
	}

	// Read the prescribed amount into our buffer.
	d.dataBuf.Reset()
	d.dataBuf.Grow(int(size))
	lr := io.LimitedReader{
		R: r,
		N: int64(size),
	}
	readCount, err := d.dataBuf.ReadFrom(&lr)
	count += readCount
	if err != nil {
		return count, err
	}

	// Load this buffer into our decoder buffer and decode the next message.
	d.buf.SetBuf(d.dataBuf.Bytes())
	return count, d.buf.Unmarshal(pb)
}
