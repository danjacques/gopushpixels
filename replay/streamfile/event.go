// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"github.com/danjacques/gopushpixels/pixel"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/protocol/pixelpusher"

	"github.com/pkg/errors"
)

// ErrEncodingNotSupported is an error returned when a requested encoding is
// not supported.
var ErrEncodingNotSupported = errors.New("packet encoding not supported")

// EncodePacket encodes a protocol packet as zero or more event packet
// messages.
//
// If the packet could not be encoded, ErrEncodingNotSupported will be
// returned.
func EncodePacket(device int64, pkt *protocol.Packet) ([]*Event_Packet, error) {
	packets, err := encodePacketWithoutDevice(pkt)
	if err != nil {
		return nil, err
	}

	// Add our device ID to each packet.
	for _, pkt := range packets {
		pkt.Device = device
	}
	return packets, nil
}

func encodePacketWithoutDevice(pkt *protocol.Packet) ([]*Event_Packet, error) {
	switch {
	case pkt.PixelPusher != nil:
		return encodePixelPusherPacket(pkt.PixelPusher)
	default:
		return nil, ErrEncodingNotSupported
	}
}

func encodePixelPusherPacket(pkt *pixelpusher.Packet) ([]*Event_Packet, error) {
	switch {
	case pkt.StripStates != nil:
		return encodePixelPusherStripStates(pkt.StripStates)

	default:
		return nil, ErrEncodingNotSupported
	}
}

func encodePixelPusherStripStates(ss []*pixelpusher.StripState) ([]*Event_Packet, error) {
	if len(ss) == 0 {
		return nil, nil
	}

	cmds := make([]*Event_Packet, len(ss))
	for i := range ss {
		cmds[i] = &Event_Packet{
			Contents: &Event_Packet_PixelpusherPixels{
				PixelpusherPixels: &PixelPusherPixels{
					StripNumber: int32(ss[i].StripNumber),
					PixelData:   ss[i].Pixels.Bytes(),
				},
			},
		}
	}

	return cmds, nil
}

// Decode decodes this Packet into its protocol component.
//
// Decode may reference byte streams in d, and the resulting Packet will only
// remain valid so long as the original d's bytes are valid. If this is a
// concern, d should be cloned prior to calling Decode to duplicate the
// buffer.
func (ec *Event_Packet) Decode(d *Device) (*protocol.Packet, error) {
	var pkt protocol.Packet

	switch t := ec.Contents.(type) {
	case nil:
		// Empty packet.
		return &pkt, nil

	case *Event_Packet_PixelpusherPixels:
		eventPixels := t.PixelpusherPixels
		if eventPixels.StripNumber >= int32(len(d.Strip)) {
			return nil, errors.Errorf("strip index %d out of bounds (%d)", eventPixels.StripNumber, len(d.Strip))
		}
		strip := d.Strip[eventPixels.StripNumber]

		// Determine our pixel buffer layout.
		var bufferLayout pixel.BufferLayout
		switch strip.PixelType {
		case Device_Strip_RGB:
			bufferLayout = pixel.BufferRGB
		case Device_Strip_RGBOW:
			bufferLayout = pixel.BufferRGBOW
		default:
			return nil, errors.Errorf("unknown pixel type: %v", strip.PixelType)
		}

		// Build our PixelBuffer.
		pp := pixelpusher.Packet{
			StripStates: []*pixelpusher.StripState{
				{
					StripNumber: pixelpusher.StripNumber(eventPixels.StripNumber),
					Pixels: pixel.Buffer{
						Layout: bufferLayout,
					},
				},
			},
		}
		pp.StripStates[0].Pixels.UseBytes(eventPixels.PixelData)
		pkt.PixelPusher = &pp
	}

	return &pkt, nil
}
