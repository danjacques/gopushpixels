// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixel

import (
	"fmt"
)

// pixelLinearExp is the linear expansion table for pixel data.
var pixelLinearExp = [256]byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4,
	5, 5, 5, 5, 5, 5, 5, 5, 6, 6, 6, 6, 6, 6, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 9, 9, 9, 9, 9, 10, 10, 10, 10, 11, 11, 11, 11, 12, 12, 12,
	13, 13, 13, 14, 14, 14, 14, 15, 15, 16, 16, 16, 17, 17, 17, 18, 18, 19, 19, 20, 20, 20, 21, 21, 22, 22, 23, 23, 24, 25, 25, 26, 26, 27,
	27, 28, 29, 29, 30, 31, 31, 32, 33, 34, 34, 35, 36, 37, 38, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 54, 55, 56, 57,
	59, 60, 61, 63, 64, 65, 67, 68, 70, 72, 73, 75, 76, 78, 80, 82, 83, 85, 87, 89, 91, 93, 95, 97, 99, 102, 104, 106, 109, 111, 114, 116,
	119, 121, 124, 127, 129, 132, 135, 138, 141, 144, 148, 151, 154, 158,
	161, 165, 168, 172, 176, 180, 184, 188, 192, 196, 201, 205,
	209, 214, 219, 224, 229, 234, 239, 244, 249, 255,
}

// P is the state of a single pixel.
//
// Depending on the strip that P belongs to, Orange and White may be
// ignored.
type P struct {
	Red   uint8
	Green uint8
	Blue  uint8

	// Note that Orange and White take three bytes in the protocol, despite the
	// fact that they can only hold a single byte value.
	//
	// These may not be populated if the strip is not an RGBOW strip.
	Orange uint8
	White  uint8
}

func (p *P) String() string {
	if p.Orange == 0 && p.White == 0 {
		return fmt.Sprintf("(%d, %d, %d)", p.Red, p.Green, p.Blue)
	}
	return fmt.Sprintf("(%d, %d, %d / %d, %d)", p.Red, p.Green, p.Blue, p.Orange, p.White)
}

// AntiLog returns a new Pixel that has been shifted against the pixelLinearExp
// luminescence shift table.
//
// This shift can increase the quality of video rendering.
func (p *P) AntiLog() P {
	return P{
		Red:    pixelLinearExp[p.Red],
		Green:  pixelLinearExp[p.Green],
		Blue:   pixelLinearExp[p.Blue],
		Orange: pixelLinearExp[p.Orange],
		White:  pixelLinearExp[p.White],
	}
}
