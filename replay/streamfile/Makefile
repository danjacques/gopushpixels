# Copyright 2018 Dan Jacques. All rights reserved.
# Use of this source code is governed under the MIT License
# that can be found in the LICENSE file.

# Makefile to regenerate the protobufs.

# Get path to this Makefile, regardless of current working directory.
CURRENT_MAKEFILE_LIST := $(MAKEFILE_LIST)
base := $(dir $(firstword $(CURRENT_MAKEFILE_LIST)))

# Path to the Go binar
GO_BINDIR := ${GOPATH}/bin
export PATH := ${GO_BINDIR}:${PATH}

# Get a list of all protobufs.
ALL_PROTOS = $(wildcard $(base)/*.proto)
ALL_GENERATED = $(patsubst %.proto,%.pb.go,$(ALL_PROTOS))

PROTO_INCLUDES :=
PROTO_INCLUDES += -I${GOPATH}/src/github.com/golang/protobuf
PROTO_INCLUDES += -I$(base)

# Setup default target.
default: protos

# Build Go protobufs from proto sources.
.PHONY: protos
protos: $(ALL_PROTOS) protoc-gen-go
	protoc $(PROTO_INCLUDES) --go_out=$(base) $(ALL_PROTOS)

# This target installs the "protoc-gen-go" utility into "GO_BINDIR".
.PHONY: protoc-gen-go
protoc-gen-go:
	go install github.com/golang/protobuf/protoc-gen-go

# Clean all generated files.
.PHONY: clean
clean:
	rm -f $(ALL_GENERATED)
