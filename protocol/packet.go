// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package protocol

import (
	"github.com/danjacques/gopushpixels/protocol/pixelpusher"
	"github.com/danjacques/gopushpixels/support/byteslicereader"
	"github.com/danjacques/gopushpixels/support/network"

	"github.com/pkg/errors"
)

// Packet is a generic interpreted command.
//
// Only one field in the Packet should be populated.
type Packet struct {
	// PixelPusher is the PixelPusher packet data. It will be populated if this
	// packet is a PixelPusher packet.
	PixelPusher *pixelpusher.Packet
}

// PacketReader reads packet structure from a stream.
//
// PacketReader is lightweight enough to be created as-needed; however, re-using
// a PacketReader allows it to re-use its underlhing buffer, which may be more
// efficient.
//
// PacketReader should generally not be created directly by the user, but
// instead populated from a device's DiscoveryHeaders' PacketReader method.
//
// PacketReader is not safe for concurrent use.
type PacketReader struct {
	// PixelPusher is the PixelPusher-specific implementation of a packet reader.
	PixelPusher *pixelpusher.PacketReader
}

// ReadPacket reads a Packet, pkt, from a source of data.
//
// If the packet could not be read, ReadPacket returns an error.
//
// The returned packet will reference data slices returned by r, and should
// not outlive the underlying buffer.
func (pr *PacketReader) ReadPacket(r *byteslicereader.R, pkt *Packet) error {
	switch {
	case pr.PixelPusher != nil:
		if pkt.PixelPusher == nil {
			pkt.PixelPusher = &pixelpusher.Packet{}
		}
		return pr.PixelPusher.ReadPacket(r, pkt.PixelPusher)

	default:
		return errors.New("packet stream is not configured")
	}
}

// PacketStream is the overall state of a packet stream.
//
// PacketStream should generally not be created directly by the user, but
// instead populated from a device's DiscoveryHeaders' PacketStream method.
//
// PacketStream is not safe for concurrent use.
type PacketStream struct {
	// pixelPusher is a PixelPusher-specific packet stream.
	PixelPusher *pixelpusher.PacketStream
}

// Send sends the contents of the specified Packet.
func (ps *PacketStream) Send(ds network.DatagramSender, pkt *Packet) error {
	switch {
	case ps.PixelPusher != nil:
		return ps.PixelPusher.Send(ds, pkt.PixelPusher)

	default:
		return errors.New("packet stream is not configured")
	}
}

// Flush flushes any buffered data to the underlying connection.
func (ps *PacketStream) Flush(ds network.DatagramSender) error {
	switch {
	case ps.PixelPusher != nil:
		return ps.PixelPusher.Flush(ds)

	default:
		return errors.New("packet stream is not configured")
	}
}
