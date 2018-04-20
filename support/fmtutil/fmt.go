// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

// Package fmtutil contains formatting helpers.
package fmtutil

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

// Hex is a byte slice that renders as a hex-dumped string.
//
// It can be used for easy lazy hex dumping.
type Hex []byte

func (h Hex) String() string { return hex.Dump([]byte(h)) }

// HexSlice is a byte slice that renders as a sequence of hex bytes, instead
// of the default decimal bytes.
//
// Output as: "[4]vbyte{0x!0, 0x20, 0x30, 0x40}"
//
// It can be used for easy lazy hex dumping.
type HexSlice []byte

func (hs HexSlice) String() string {
	var sb bytes.Buffer
	sb.Grow((6 * len(hs)) + 16) // 16 is more than we need for static content.
	fmt.Fprintf(&sb, "[%d]byte{", len(hs))
	for i, b := range hs {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "0x%02X", b)
	}
	sb.WriteString("}")
	return sb.String()
}
