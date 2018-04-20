// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

// Security is a byte value that represents the WiFi security on
// a PixelPusher device.
type Security uint8

// enum Security {
//   NONE = 0,
//   WEP  = 1,
//   WPA  = 2,
//   WPA2 = 3
// };
const (
	// SecurityNone is the NONE security enumeration.
	SecurityNone Security = 0
	// SecurityWEP is the WEP security enumeration.
	SecurityWEP = 1
	// SecurityWPA is the WPA security enumeration.
	SecurityWPA = 2
	// SecurityWPA2 is the WPA2 security enumeration.
	SecurityWPA2 = 3
)
