// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package dataio

import (
	"io"
)

// Writer represents a Writer that can write both individual bytes and
// sequences of bytes.
type Writer interface {
	io.Writer
	io.ByteWriter
}

// MakeWriter returns a Writer for the specified Writer.
func MakeWriter(w io.Writer) Writer {
	if dr, ok := w.(Writer); ok {
		return dr
	}
	return &simulatedWriter{w}
}

type simulatedWriter struct {
	io.Writer
}

func (w *simulatedWriter) WriteByte(c byte) error {
	d := [1]byte{c}
	switch amt, err := w.Write(d[:]); {
	case err != nil:
		return err
	case amt != 1:
		panic("invalid Writer implementation")
	default:
		return nil
	}
}
