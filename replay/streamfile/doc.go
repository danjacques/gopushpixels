// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

// Package streamfile defines a file format to record and replay packet stream
// files.
//
// Stream files use Google's protobuf library. Each stream data block is
// composed of a series of protobuf messages, managed by the protostream
// package.
//
// A stream "file" consists of three types of components:
//
//	- A base directory, which is considered the file's path and whiel contains
//	  all of the streamfile's data.
//	- A metadata protobuf file, stored in text protobuf format, which describes
//	  the layout of the file and its stream data.
//	- One or more event files, which contain raw binary stream data.
//
// A stream file is constructed through atomic filesystem operations. During
// recording, it is built in a temporary directory. When complete, it is
// finished and then moved into its destination.
//
// Stream files can be trivially merged together to form a composite stream
// file. This is done by hard-linking (or, failing that, copying) the event
// files from the merged streamfile into a new directory and then merging the
// metadata file.
//
// streamfile supports compression. This is fairly necessary, as raw stream data
// can be huge:
//
//	- SNAPPY compression uses Google's Snappy compression algorithm for CPU-
//	  friendly reads and writes with a decent compression ratio.
//	- GZIP uses the gzip library. This is more CPU intensive but also more
//	  efficient than SNAPPY.
//	- RAW avoids compression altogether. This may be useful when the underlying
//	  filesystem  offers compression, as "btrfs" does.
//
// ## Regenerating Protobufs
//
// When changes to the ".proto" files are made, the generated protobuf Go code
// must be regenerated. This is done through a Makefile, but is wired into
// "go:generate" for convenience.
//
// Since it requires non-Go components, protobuf regeneration is gated on the
// "protoc" build tag. It can be run by calling:
//
//	go generate -tags 'protoc' ./replay/streamfile/...
package streamfile
