// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

// Generator file to build the contained protobufs.
//
// Generation uses a Makefile, which in turn runs the actual compiler.
//
// On Debian, this is the "protobuf-compiler" package.

// +build protoc

//go:generate make

package streamfile

import (
	_ "github.com/golang/protobuf/proto"
)
