// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

// CompressionFlag is a pflag.Value implementation that stores a compression
// value.
type CompressionFlag Compression

var _ pflag.Value = (*CompressionFlag)(nil)

func (cf *CompressionFlag) String() string { return Compression(*cf).String() }

// Set implements pflag.Value.
func (cf *CompressionFlag) Set(v string) error {
	if cv, ok := Compression_value[v]; ok {
		*cf = CompressionFlag(cv)
		return nil
	}
	return errors.Errorf("unknown compression type: %q", v)
}

// Type implements pflag.Value.
func (cf *CompressionFlag) Type() string { return "streamfile.Compression" }

// Value returns the compression value held by this flag.
func (cf CompressionFlag) Value() Compression { return Compression(cf) }

// CompressionFlagValues returns the list of possible values for a
// CompressionFlag.
func CompressionFlagValues() string {
	type entry struct {
		value int32
		name  string
	}

	// Get all available options, sorted by their enumeration value.
	entries := make([]entry, 0, len(Compression_value))
	for value, name := range Compression_name {
		entries = append(entries, entry{value, name})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].value < entries[j].value })

	// Get their associated name strings.
	opts := make([]string, len(entries))
	for i := range entries {
		opts[i] = entries[i].name
	}
	return strings.Join(opts, ", ")
}
