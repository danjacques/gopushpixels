// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"github.com/danjacques/gopushpixels/pixel"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/protocol/pixelpusher"
)

// Mutable wraps a device, D, offering a method of setting and updating its
// pixel state.
//
// Currently, Mutable is only implemented for PixelPusher devices.
//
// Mutable is not safe for concurrent use; concurrent users must lock around it.
type Mutable struct {
	deviceType     protocol.DeviceType
	strips         []mutableStripState
	pixelsPerStrip int
}

// NumStrips returns the number of configured strips.
func (m *Mutable) NumStrips() int { return len(m.strips) }

// PixelsPerStrip returns the number of pixels per strip.
func (m *Mutable) PixelsPerStrip() int { return m.pixelsPerStrip }

// SetPixel sets the value of the specified pixel in the specified strip to v.
//
// If the pixel exists and was modified, SetPixel will return true and note that
// it has been mutated.
//
// If index is out of bounds, SetPixel will not change anything and return
// false.
func (m *Mutable) SetPixel(strip, pixel int, v pixel.P) bool {
	switch {
	case strip < 0 || strip >= len(m.strips):
		return false
	case pixel < 0 || pixel >= m.pixelsPerStrip:
		return false
	}

	ss := &m.strips[strip]
	if ss.Pixels.Pixel(pixel) != v {
		ss.Pixels.SetPixel(pixel, v)
		ss.modified = true
		return true
	}
	return false
}

// GetPixel returns the value of the specified pixel offset in the specified
// strip.
//
// If p or strip are out of bounds, a zero-value pixel will be returned.
func (m *Mutable) GetPixel(strip, p int) pixel.P {
	if strip < 0 || strip >= len(m.strips) {
		return pixel.P{}
	}
	return m.strips[strip].Pixels.Pixel(p)
}

// SetPixels sets the full set of pixels for this device to the value in pixels.
//
// If pixels has the same flags and size as m's strip,  SetPixels will be a fast
// buffer clone. Otherwise, pixels will be set pixel-by-pixel within bounds.
//
// If strip is out of bounds, nothing will be updated.
func (m *Mutable) SetPixels(strip int, pixels *pixel.Buffer) {
	if strip < 0 || strip >= len(m.strips) {
		return
	}

	// Fast path: pixels lines up perfectly. This should be the case most of the
	// time.
	mst := &m.strips[strip]
	mst.Pixels.CopyPixelValuesFrom(pixels)
	mst.modified = true
}

// ClonePixelsTo clones the contents of the specified pixel strip into target.
//
// If strip references an invalid strip index, nothing will happen, and target
// will be unmodified.
func (m *Mutable) ClonePixelsTo(strip int, target *pixel.Buffer) {
	if strip < 0 || strip >= len(m.strips) {
		return
	}
	target.CloneFrom(&m.strips[strip].Pixels)
}

// SyncPacket generates a packet containing an update instruction for each
// modified strip. All strips will be marked unmodified after sync.
func (m *Mutable) SyncPacket() *protocol.Packet {
	// We'll only instantiate the Packet if we have a modified strip.
	var pp *pixelpusher.Packet
	for i := range m.strips {
		mst := &m.strips[i]
		if !mst.modified {
			continue
		}

		if pp == nil {
			pp = &pixelpusher.Packet{
				StripStates: make([]*pixelpusher.StripState, 0, len(m.strips)),
			}
		}
		ss := pixelpusher.StripState{
			StripNumber: pixelpusher.StripNumber(i),
		}
		ss.Pixels.CloneFrom(&mst.Pixels)
		pp.StripStates = append(pp.StripStates, &ss)
		mst.modified = false
	}

	// If no strips were modified, return nil.
	if pp == nil {
		return nil
	}

	return &protocol.Packet{
		PixelPusher: pp,
	}
}

// Initialize ensures that each strip state matches the state described
// by the device's discovery headers.
//
// Initialize can be called more than once, and will adjust the current strip
// and pixel count based on the current set of headers.
func (m *Mutable) Initialize(dh *protocol.DiscoveryHeaders) {
	m.deviceType = dh.DeviceType
	pp := dh.PixelPusher

	if m.deviceType != protocol.PixelPusherDeviceType || pp == nil {
		// Non-PixelPusher devices aren't supported.
		m.strips = nil
		return
	}

	wantNumStrips := int(pp.StripsAttached)
	if len(m.strips) != wantNumStrips {
		// Allocate the proper number of strips.
		newStrips := make([]mutableStripState, wantNumStrips)
		copy(newStrips, m.strips[:len(m.strips)])
		m.strips = newStrips
	}

	// Initialize / resize the remaining strips. If we do need to resize, this
	// will zero the buffer.
	m.pixelsPerStrip = int(pp.PixelsPerStrip)
	for i := range m.strips {
		mst := &m.strips[i]
		if mst.StripState == nil {
			mst.StripState = &pixelpusher.StripState{
				StripNumber: pixelpusher.StripNumber(i),
			}
			mst.modified = true
		}

		// Initialize or update our pixel state.
		//
		// Note that we have to test "Len" before changing any flags, as flag
		// changes may invalidate the buffer offsets.
		resetPixels := false
		if mst.Pixels.Len() != m.pixelsPerStrip {
			resetPixels = true
		}
		if l := pp.StripFlags[i].PixelBufferLayout(); mst.Pixels.Layout != l {
			mst.Pixels.Layout = l
			resetPixels = true
		}

		if resetPixels {
			mst.Pixels.Reset(m.pixelsPerStrip)
			mst.modified = true
		}
	}
}

type mutableStripState struct {
	*pixelpusher.StripState
	modified bool
}
