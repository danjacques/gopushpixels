// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

// Package byteslicereader offers R a slice-backed Reader that offers zero-copy
// options.
//
// Standard io.Reader methods require that data be copied into a target Buffer.
// The zero-copy options, Peek and Next, allow for data to be returned as slices
// of R's underlying Buffer.
//
// With great power comes great responsibility: holding a reference to an
// underlying Buffer means that the Buffer must persist as long as that
// reference is valid, and that modifications to that reference must be
// coordinated with any other consumers.
//
// R allows for APIs that may want to be zero-copy conditionally by exposing
// an AlwaysCopy flag. If set, R's zero-copy operations will return copies of
// the underlying Buffer, decoupling them from their base state. Receivers that
// accept R instances can use it without any modifications.
package byteslicereader

import (
	"io"

	"github.com/pkg/errors"
)

// R is an io.Reader-inspired interface that exposes operations
// that return on byte slices, instead of filling a byte slice.
//
// This allows for efficient zero-copy read operations by returning sections of
// a backing array.
//
// This is more efficient than copying the data, but carries the peril that, for
// non-copying calls, the returned data is not independent from the reader's
// Buffer. Caution must be taken to ensure that references to the underlying
// Buffer do not persist when/if the buffer is reallocated for other purposes.
//
// Writing to returned Buffer segments is allowed, but caution must be taken not
// to hand the a section of Buffer to two entities when one of them might write
// to it.
//
// R can act like an io.Reader and io.ByteReader, allowing it to
// interface with other APIs at the expense of introducing data copying. This
// may be acceptable for small amounts of data.
//
// R can be copied, creating a snapshot of its current state.
type R struct {
	// Buffer is the backing buffer for this reader.
	Buffer []byte

	// AlwaysCopy, if true, causes zero-copy methods to return copies of their
	// backing data instead of direct references. This can be set to cause methods
	// that operate on R to automatically own an independent set
	// of data, but obviates the performance benefits.
	//
	// All zero-copy methods honor AlwaysCopy, so it is safe to assume that data
	// returned by all R methods is owned by the caller when
	// AlwaysCopy is true.
	AlwaysCopy bool

	// pos is the R's position within Buffer.
	pos int64
}

var _ interface {
	io.Reader
	io.ByteReader
	io.Seeker
} = (*R)(nil)

func (r *R) remainingSlice() []byte {
	if r.pos >= int64(len(r.Buffer)) {
		return nil
	}
	return r.Buffer[r.pos:]
}

// Remaining returns the number of bytes remaining in the reader, from the
// current position.
func (r *R) Remaining() int { return len(r.remainingSlice()) }

// Read implements io.Reader.
//
// Note that using Read cause data to be copied.
func (r *R) Read(b []byte) (amt int, err error) {
	remaining := r.remainingSlice()
	amt = copy(b, remaining)

	r.pos += int64(amt)
	if r.pos >= int64(len(r.Buffer)) {
		err = io.EOF
	}
	return
}

// ReadByte implements io.ByteReader.
func (r *R) ReadByte() (b byte, err error) {
	if r.pos >= int64(len(r.Buffer)) {
		return 0, io.EOF
	}

	b, r.pos = r.Buffer[r.pos], r.pos+1
	return
}

// Seek implements io.Seeker.
func (r *R) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekEnd:
		newPos = offset + int64(len(r.Buffer)) - 1
		if offset > 0 {
			// Seeking to any positive offset is legal.
			if len(r.Buffer) == 0 {
				r.pos = offset
			} else {
				r.pos = newPos
			}
			return r.pos, nil
		}
	case io.SeekCurrent:
		newPos = r.pos + offset
	}

	if newPos < 0 || newPos >= int64(len(r.Buffer)) {
		return r.pos, errors.New("seek outside of bounds")
	}

	r.pos = newPos
	return r.pos, nil
}

// Peek returns the next n bytes in r without advancing it.
//
// Peek is a zero-copy method, and returns a slice of the underlying Buffer
// unless AlwaysCopy is true.
//
// If there are fewer than n bytes in r, Peek will return as many as possible.
func (r *R) Peek(n int) []byte {
	v := r.remainingSlice()
	if n < len(v) {
		v = v[:n]
	}

	if r.AlwaysCopy {
		v = append([]byte(nil), v...)
	}

	return v
}

// PeekByte is like Peek, but it returns a single byte.
func (r *R) PeekByte() (byte, error) {
	remaining := r.remainingSlice()
	if len(remaining) > 0 {
		return remaining[0], nil
	}
	return 0, io.EOF
}

// Next returns the next n bytes in r, advancing r.
//
// Next is a zero-copy equivalent to Read, and returns a slice of the underlying
// Buffer unless AlwaysCopy is true.
//
// If there are fewer than n bytes in r, Next will return as many bytes as it
// can and io.EOF as an error. Next will never return an error if all requested
// bytes are returned.
func (r *R) Next(n int) (v []byte, err error) {
	v = r.remainingSlice()
	if n < len(v) {
		v = v[:n]
	} else {
		err = io.EOF
	}

	if r.AlwaysCopy {
		v = append([]byte(nil), v...)
	}

	r.pos += int64(len(v))
	return
}
