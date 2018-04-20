// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package dataio

import (
	"io"
)

// Reader represents a Reader that can read both individual bytes and
// sequences of bytes.
type Reader interface {
	io.Reader
	io.ByteReader
}

// MakeReader returns a Reader for the specified Reader.
func MakeReader(r io.Reader) Reader {
	if dr, ok := r.(Reader); ok {
		return dr
	}
	return &simulatedReader{r}
}

type simulatedReader struct {
	io.Reader
}

func (r *simulatedReader) ReadByte() (v byte, err error) {
	var d [1]byte
	var amt int

	amt, err = r.Read(d[:])
	if amt == 1 {
		v = d[0]
	}
	return
}
