// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	"fmt"
	"strings"

	"github.com/danjacques/gopushpixels/pixel"
)

// StripType is a PixelPusher strip type enumeration.
type StripType uint8

const (
	// StripLPD8806 is the LPD8806 strip type.
	StripLPD8806 StripType = 0
	// StripWS2801 is the WS2801 strip type.
	StripWS2801 = 1
	// StripWS2811 is the WS2811 strip type.
	StripWS2811 = 2
	// StripAPA102 is the APA102 strip type.
	StripAPA102 = 3
)

// StripNumber is the number assigned to an individual Strip.
type StripNumber uint8

// PixelPusher strip flags.
const (
	// SFlagRGBOW is the SFLAG_RGBOW strip flag.
	SFlagRGBOW StripFlags = (1 << iota)
	// SFlagWidePixels is the SFLAG_WIDEPIXELS strip flag.
	SFlagWidePixels
	// SFlagLogarithmic is the SFLAG_LOGARITHMIC strip flag.
	SFlagLogarithmic
	// SFlagMotion is the SFLAG_MOTION strip flag.
	SFlagMotion
	// SFlagNotIdempotent is the SFLAG_NOTIDEMPOTENT strip flag.
	SFlagNotIdempotent
	// SFlagBrightness is the SFLAG_BRIGHTNESS strip flag.
	SFlagBrightness
	// SFlagMonochrome is the SFLAG_MONOCHROME strip flag.
	SFlagMonochrome
)

var flagNames = []struct {
	flag StripFlags
	text string
}{
	{SFlagRGBOW, "RGBOW"},
	{SFlagWidePixels, "WIDEPIXELS"},
	{SFlagLogarithmic, "LOGARITHMIC"},
	{SFlagMotion, "MOTION"},
	{SFlagNotIdempotent, "NOTIDEMPOTENT"},
	{SFlagBrightness, "BRIGHTNESS"},
	{SFlagMonochrome, "MONOCHROME"},
}

// StripState is the pixel state of a single strip.
type StripState struct {
	// StripNumber is the strip number that this state represents.
	StripNumber StripNumber

	// Pixels is the set of pixels that belongs to this strip.
	Pixels pixel.Buffer
}

// StripFlags represents information about a PixelPusher Strip.
//
// TODO: Add other pieces of information from flags.
type StripFlags uint8

// IsRGBOW is true if the SFLAG_RGBOW strip flag is enabled.
//
// If true, the packet stream uses RGBOW encoding. If false, the stream uses
// RGB encoding.
func (sf StripFlags) IsRGBOW() bool { return sf.getFlag(SFlagRGBOW) }

// SetRGBOW sets the value of the SFLAG_RGBOW strip flag.
func (sf *StripFlags) SetRGBOW(v bool) { sf.setFlag(SFlagRGBOW, v) }

// PixelBufferLayout returns the pixel buffer layout to use for this strip.
func (sf *StripFlags) PixelBufferLayout() pixel.BufferLayout {
	if sf.IsRGBOW() {
		return pixel.BufferRGBOW
	}
	return pixel.BufferRGB
}

// String writes a string version of these flags.
//
// Output looks like:
// 0x03(RGBOW|WIDEPIXELS)
func (sf StripFlags) String() string {
	flags := make([]string, 0, 7)
	for _, f := range flagNames {
		if sf.getFlag(f.flag) {
			flags = append(flags, f.text)
		}
	}

	if len(flags) > 0 {
		return fmt.Sprintf("0x%02X(%s)", uint8(sf), strings.Join(flags, "|"))
	}
	return fmt.Sprintf("0x%02X", uint8(sf))
}

func (sf StripFlags) getFlag(flag StripFlags) bool { return (sf & flag) != 0 }
func (sf *StripFlags) setFlag(flag StripFlags, v bool) {
	if v {
		*sf = *sf | flag
	} else {
		*sf = *sf & (^flag)
	}
}
