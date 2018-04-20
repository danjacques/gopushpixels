// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixel

import (
	"io"

	"github.com/danjacques/gopushpixels/support/byteslicereader"

	"github.com/pkg/errors"
)

// BufferLayout is the layout of a pixel buffer.
type BufferLayout int

const (
	// BufferRGB is a BufferLayout specifying a series of contiguous (R, G, B)
	// pixel value bytes.
	BufferRGB BufferLayout = iota
	// BufferRGBOW is a BufferLayout specifying a series of contiguous
	// (R, G, B, OOO, WWW) pixel value bytes. The O and W bytes are each three
	// bytes of the same value.
	BufferRGBOW
)

// Buffer represents the wire format for a series of consecutive pixels.
// It is used for minimal-copy pixel processing.
type Buffer struct {
	// Layout is the buffer layout to use.
	//
	// Adjusting this value will invalidate the current buffered data. The user
	// must call Reset afterwards.
	Layout BufferLayout

	buf []byte
}

// Len returns the number of pixels allocated in pb.
func (pb *Buffer) Len() int { return len(pb.buf) / pb.pixelSize() }

// Reset clears the buffer and allocates room for size pixels.
//
// If the underlying buffer is already >= this size, it will be reused;
// otherwise, a new buffer will be allocated.
func (pb *Buffer) Reset(size int) {
	pb.resetBuffer(size, true)
}

// UseBytes loads buf directly into this Buffer. This creates a
// functional Buffer with no copying.
//
// Note that buf may be retained and used by pb indefnitely, and should not be
// reused while pb is active. Loading a new buffer using UseBytes will cause
// pb to stop using buf.
func (pb *Buffer) UseBytes(buf []byte) { pb.buf = buf }

func (pb *Buffer) resetBuffer(size int, zero bool) {
	bytesNeeded := size * pb.pixelSize()
	if cap(pb.buf) < bytesNeeded {
		// Create a new buffer.
		pb.buf = make([]byte, bytesNeeded)
	} else {
		// Size and zero the existing buffer.
		pb.buf = pb.buf[:bytesNeeded]
		for i := range pb.buf {
			pb.buf[i] = 0
		}
	}
}

// ReadFrom reads a Buffer with size pixels from the supplied byte slice
// reader.
//
// The Buffer's underlying data will be a pointer into that
// byte slice reader's buffer, and pb expects to take ownership over that section
// of the buffer.
func (pb *Buffer) ReadFrom(r *byteslicereader.R, size int) error {
	pixelBufSize := size * pb.pixelSize()
	buf, err := r.Next(pixelBufSize)

	// Handle case where completing the buffer hits EOF.
	if err == io.EOF && len(buf) == pixelBufSize {
		err = nil
	}
	if err != nil {
		return errors.Wrapf(err, "could not read pixel buffer (size %d)", pixelBufSize)
	}

	pb.buf = buf
	return nil
}

// CloneFrom clones the state of other efficiently.
func (pb *Buffer) CloneFrom(other *Buffer) { pb.CloneFromWithLen(other, other.Len()) }

// CloneFromWithLen clones pb from the pixels in other. pb will have count
// pixels. If other has additional pixels, they will be discarded during the
// clone. If other has fewer pixels, pb's remainder will be left uninitialised
// (black).
func (pb *Buffer) CloneFromWithLen(other *Buffer, count int) {
	pb.Layout = other.Layout

	// Reset our buffer to the appropriate number of pixels. We don't' need to
	// zero our buffer since we're going to overwrite it anyway.
	pb.resetBuffer(count, false)

	// Copy pixel data from other. Since we've synchornized flags, we are
	// byte-for-byte compatible with other, and can copy thusly.
	copySize := len(pb.buf)
	if copySize > len(other.buf) {
		copySize = len(other.buf)
	}
	zeroStart := copy(pb.buf, other.buf[:copySize])

	// Zero the remainder of the buffer. This happens when count > other.Len().
	for i := zeroStart; i < len(pb.buf); i++ {
		pb.buf[i] = 0x00
	}
}

// CopyPixelValuesFrom tries to set pb's pixel values to match those in other.
//
// If pb's flags and size align with other, this will be a fast buffer copy.
// Otherwise, as many pixels as possible will be copied from other one-by-one.
//
// In either case, pb will not have its flags or size changed.
func (pb *Buffer) CopyPixelValuesFrom(other *Buffer) {
	// If our flags match, do a direct clone.
	if pb.Layout == other.Layout {
		pb.CloneFromWithLen(other, pb.Len())
		return
	}

	// Copy min(len(pb), len(other)) pixels one-by-one.
	numCopy := pb.Len()
	if l := other.Len(); l < numCopy {
		numCopy = l
	}

	for i := 0; i < numCopy; i++ {
		pb.SetPixel(i, other.Pixel(i))
	}
}

// Bytes returns the raw bytes for this buffer.
func (pb *Buffer) Bytes() []byte { return pb.buf }

// Pixel returns the pixel data for the Pixel at index i.
//
// If i is out of bounds, Pixel will return a zero value.
func (pb *Buffer) Pixel(i int) (p P) {
	offset := i * pb.pixelSize()
	if offset < 0 || offset >= len(pb.buf) {
		return
	}

	// Both use the first three indices as RGB.
	p.Red, p.Green, p.Blue = pb.buf[offset], pb.buf[offset+1], pb.buf[offset+2]

	switch pb.Layout {
	case BufferRGBOW:
		p.Orange = pb.buf[offset+3]
		p.White = pb.buf[offset+6]
	}
	return
}

// SetPixel sets the pixel value at index i.
//
// If i is out of bounds, SetPixel will do nothing.
func (pb *Buffer) SetPixel(i int, p P) {
	offset := i * pb.pixelSize()
	if offset < 0 || offset >= len(pb.buf) {
		return
	}

	// Both use the first three indices as RGB.
	pb.buf[offset], pb.buf[offset+1], pb.buf[offset+2] = p.Red, p.Green, p.Blue

	switch pb.Layout {
	case BufferRGBOW:
		pb.buf[offset+3], pb.buf[offset+4], pb.buf[offset+5] = p.Orange, p.Orange, p.Orange
		pb.buf[offset+6], pb.buf[offset+7], pb.buf[offset+8] = p.White, p.White, p.White
	}
}

// SetPixels sets the Buffer's content to the set of pixels provided.
func (pb *Buffer) SetPixels(pixels ...P) {
	// Reset the buffer to the appropriate size. No need to zero since we're
	// filling everything in.
	pb.resetBuffer(len(pixels), false)

	for i, p := range pixels {
		pb.SetPixel(i, p)
	}
}

// AntiLog performs an antilog transform on every pixel in the buffer. This is
// more efficient than calling AntiLog on each individual pixel.
func (pb *Buffer) AntiLog() {
	pixelIndex := 0
	for i := 0; i < len(pb.buf); i++ {
		pb.buf[i] = pixelLinearExp[pb.buf[i]]

		// RGBOW uses junk data for its repeated OO and WW pixels, so skip them.
		switch pb.Layout {
		case BufferRGB:
			if pixelIndex > 2 {
				// Reset index every 3 pixels.
				pixelIndex = 0
			}

		case BufferRGBOW:
			if pixelIndex > 3 {
				i += 2 // Skip duplicate 2 pixels.
				pixelIndex++
			}

			if pixelIndex > 4 {
				// Reset index every 5 pixels.
				pixelIndex = 0
			}
		}
	}
}

func (pb *Buffer) pixelSize() int {
	switch pb.Layout {
	case BufferRGB:
		return 3 // [RGB]

	case BufferRGBOW:
		// NOTE: No idea why the protocol duplicates "O" and "W" bytes. Seems like
		// a waste? Maybe forward-thinking for 24-bit O/W values in the future?
		return 9 // [RGB] [OOO] [WWW]

	default:
		panic(errors.Errorf("unknown buffer layout: %v", pb.Layout))
	}
}
