// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"time"
)

// EventStreamConfig is a configuration for the generation of event streams.
type EventStreamConfig struct {
	// WriterCompression is the compression to use when writing a file.
	WriterCompression Compression
	// WriterCompressionLevel is the compression level to apply to
	// WriterCompression, if applicable.
	WriterCompressionLevel int

	// TempDir is the temporary directory to use.
	TempDir string

	// NowFunc, if not nil, is the function to use to get the current time. If
	// nil, time.Now will be used.
	NowFunc func() time.Time
}

func (cfg *EventStreamConfig) now() time.Time {
	if cfg.NowFunc != nil {
		return cfg.NowFunc()
	}
	return time.Now()
}
