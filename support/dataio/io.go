// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package dataio

import (
	"io"
)

// ReadFull reads from r until buf is full, or until an error is encountered.
//
// This accommodates the fact that io.Reader is allowed to return less than the
// full buffer size without erroring.
func ReadFull(r io.Reader, buf []byte) error {
	// Read until we fill our buffer or encounter an error.
	for remaining := buf; len(remaining) > 0; {
		amt, err := r.Read(remaining)
		remaining = remaining[amt:]
		if err != nil {
			if err == io.EOF && len(remaining) == 0 {
				// Finished read and returned EOF.
				return nil
			}

			// Either did not finish read, or returned a non-EOF error.
			return err
		}
	}
	return nil
}
