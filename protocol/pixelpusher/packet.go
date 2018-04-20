// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/danjacques/gopushpixels/support/byteslicereader"
	"github.com/danjacques/gopushpixels/support/network"

	"github.com/pkg/errors"
)

var errStripDataTooLarge = errors.New("strip data too large")

// Packet is a single PixelPusher packet data block.
type Packet struct {
	// ID is this packet's ID.
	ID uint32

	// Command is the command for this packet.
	//
	// A Packet may provide exactly one Command or StripStates value.
	Command Command

	// StripStates is the series of strip states represented by this packet.
	//
	// A Packet may provide exactly one Command or StripStates value.
	StripStates []*StripState
}

// PacketReader reads packet structure from a stream.
type PacketReader struct {
	// PixelsPerStrip is the number of pixels belonging to a given strip.
	PixelsPerStrip int

	// StripFlags contains information about individual PixelPusher strips.
	//
	// Strip information is needed when decoding packets, since each strip's
	// encoding depends on its configuration.
	StripFlags []StripFlags
}

// ReadPacket reads a Packet, pkt, from a source of data.
//
// If the packet could not be read, ReadPacket returns an error.
//
// The returned packet will reference data slices returned by r, and should
// not outlive the underlying buffer.
func (pr *PacketReader) ReadPacket(r *byteslicereader.R, pkt *Packet) error {
	// [0:3] Read the packet index.
	if err := binary.Read(r, binary.BigEndian, &pkt.ID); err != nil {
		return err
	}

	// We need to determine if this is a command or pixel packet. We do this by
	// scanning the next series of bytes to see if they match CommandMagic.
	if bytes.Equal(r.Peek(len(CommandMagic)), CommandMagic) {
		// Consume the command magic header.
		if _, err := r.Seek(int64(len(CommandMagic)), io.SeekCurrent); err != nil {
			return err
		}

		// Read the command from the buffer.
		var err error
		pkt.Command, err = ReadCommand(r, false)
		return err
	}

	// We are reading a pixel packet.
	//
	// Read strip states until we've consumed all packet data.
	pkt.StripStates = nil
	for {
		// [0] Read the strip number.
		//
		// If this hits EOF, there are no more strips, and this is OK. Any
		// successive EOF is an incomplete strip and is an error.
		stripNumber, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// If this ID exceeds our StripFlags count, error.
		if int(stripNumber) >= len(pr.StripFlags) {
			return errors.Errorf("strip index %d exceeds maximum (%d)", stripNumber, len(pr.StripFlags)-1)
		}
		flags := pr.StripFlags[stripNumber]

		// Add a new state to our StripStates.
		state := StripState{
			StripNumber: StripNumber(stripNumber),
		}

		// Read pixel data.
		state.Pixels.Layout = flags.PixelBufferLayout()
		if err := state.Pixels.ReadFrom(r, pr.PixelsPerStrip); err != nil {
			return err
		}
		pkt.StripStates = append(pkt.StripStates, &state)
	}
}

// PacketStream is the overall state of a packet stream.
//
// A PacketStream is generally not created by a user, but rather obtained from
// a discovery Device instance's PacketStream method.
type PacketStream struct {
	// MaxStripsPerPacket is the maximum number of strip states that can be sent
	// in a single packet. This can be obtained from the device's DeviceHeader.
	MaxStripsPerPacket uint8

	// PixelsPerStrip is the number of pixels per strip.
	//
	// If this is >0, and a pixel packet is submitted with a different number of
	// pixels, it will be corrected to conform t this limit.
	PixelsPerStrip uint16

	// FixedSize, if >0, means that this packet must have the specified fixed
	// size, regardless of its contents.
	//
	// If the packet that is written exceeds this size, FixedSize will be ignored;
	// otherwise, the packet will be padded to write FixedSize total bytes.
	//
	// This value should be set based on the device's PusherFlags.
	FixedSize int

	// NextID is the next ID to assign to a packet.
	NextID uint32

	// commandBuf is a reusable buffer for building command packets.
	commandBuf bytes.Buffer

	stripStateBuf   bytes.Buffer
	stripStateCount int
}

// Send sends the contents of the specified Packet.
//
// The Packet's ID field is ignored in favor of the PacketStream generating its
// own ID.
//
// Send is a proxy for SendCommand or SendOrEnqueueStripState based on the
// contents of the packet.
func (ps *PacketStream) Send(ds network.DatagramSender, pkt *Packet) error {
	switch {
	case pkt.Command != nil:
		return ps.SendCommand(ds, pkt.Command)
	case len(pkt.StripStates) > 0:
		for _, ss := range pkt.StripStates {
			if err := ps.SendOrEnqueueStripState(ds, ss); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

// SendCommand sends a Command packet to the PacketStream's connection.
func (ps *PacketStream) SendCommand(ds network.DatagramSender, cmd Command) error {
	ps.resetPacketBuffer(&ps.commandBuf)

	// Write a command packet.
	if err := WriteCommand(cmd, &ps.commandBuf, true); err != nil {
		return err
	}

	// Send the packet.
	return ps.finalizeAndSendPacket(ds, &ps.commandBuf)
}

// SendOrEnqueueStripState enqueues a StripState to be sent.
//
// Enqueueing a StripState may cause a previously-enqueued StripState to be
// sent in order to make room for the new StripState. At most one send operation
// will occur per call, and that error value will be returned.
func (ps *PacketStream) SendOrEnqueueStripState(ds network.DatagramSender, ss *StripState) error {

	// If this StripState is empty, do nothing.
	data := ss.Pixels.Bytes()
	if len(data) == 0 {
		return nil
	}

	// If the packet has a different number of pixels per strip, conform it to
	// our requirement.
	//
	// Since we can't modify ss, we will create ae clone and send that. This
	// incurs additional expense.
	if ps.PixelsPerStrip > 0 && ss.Pixels.Len() != int(ps.PixelsPerStrip) {
		clone := StripState{
			StripNumber: ss.StripNumber,
		}
		clone.Pixels.CloneFromWithLen(&ss.Pixels, int(ps.PixelsPerStrip))
		ss = &clone
	}

	// If this would be the first strip state in the buffer, reset the buffer.
	if ps.stripStateCount == 0 {
		ps.resetPacketBuffer(&ps.stripStateBuf)
	}

	// Apply our maximum packet size constraint.
	if mps := ps.calculateMaxPacketSize(ds); mps > 0 {
		// Calculate how much extra data this strip state will add to our packet.
		stripDataSize := 1 + len(data) // [Strip#] + [Data...]

		if ps.stripStateBuf.Len()+stripDataSize > mps {
			// If we don't have any buffered data, then this single strip state is
			// too large.
			if ps.stripStateCount == 0 {
				return errStripDataTooLarge
			}

			// We have buffered data. Flush and try again.
			if err := ps.Flush(ds); err != nil {
				return err
			}

			if ps.stripStateBuf.Len()+stripDataSize > mps {
				return errStripDataTooLarge
			}
		}
	}

	// Add ss to the strip state buffer. We have already applied the size
	// constraint, so we assume it fits. We also know that doing so will not
	// exceed our maximum strips per packet, since otherwise the previous write
	// would have sent that packet.
	//
	// An edge case is when there is one strip per packet; however, this will
	// never hit the flush-before-buffering case above, since the packet will
	// never have been buffered.

	// [0] Strip number.
	ps.stripStateBuf.WriteByte(byte(ss.StripNumber))

	// [1...] Strip state data.
	ps.stripStateBuf.Write(data)

	// We didn't previously flush. Consider flushing now if we're reached a
	// constraint.
	ps.stripStateCount++
	if ps.stripStateCount >= int(ps.MaxStripsPerPacket) {
		return ps.Flush(ds)
	}

	// Buffer this packet until a future send or flush operation.
	return nil
}

// Flush flushes any pending strip states.
func (ps *PacketStream) Flush(ds network.DatagramSender) error {
	if ps.stripStateCount == 0 {
		// Nothing to flush.
		return nil
	}

	// Send the packet. Always reset our strip states regardless of whether or not
	// this was successful.
	err := ps.finalizeAndSendPacket(ds, &ps.stripStateBuf)
	if err != nil {
		return err
	}

	// Clear our strip state count. The buffer will be reset next send.
	ps.stripStateCount = 0
	return nil
}

func (ps *PacketStream) calculateMaxPacketSize(ds network.DatagramSender) (mps int) {
	mps = ps.FixedSize
	if v := ds.MaxDatagramSize(); mps <= 0 || mps > v {
		mps = v
	}
	return
}

// resetPacketBuffer clears buf and ensures that a 4-byte space is allocated at
// the beginning for the sequence number. After completion, buf can have a
// PixelPusher packet appended to it.
//
// The 4-byte space is used in finalizeAndSendPacket when the packet is sent.
func (ps *PacketStream) resetPacketBuffer(buf *bytes.Buffer) {
	if buf.Len() < 4 {
		buf.Reset()
		buf.Write([]byte{0x00, 0x00, 0x00, 0x00})
	} else {
		buf.Truncate(4)
	}
}

// finalizePacket finalizes the packet and returns the resulting packet bytes.
//
// finalizePacket assumes that buf has been reset and has the 4-byte sequence
// space reserved at the beginning.
func (ps *PacketStream) finalizeAndSendPacket(ds network.DatagramSender, buf *bytes.Buffer) error {
	// Write our packet ID to the beginning of the buffer.
	binary.BigEndian.PutUint32(buf.Bytes()[:4], ps.NextID)

	// Confirm that the final packet is capable of being sent over our connection.
	if mds := ds.MaxDatagramSize(); buf.Len() > mds {
		return errors.Errorf("packet size %d exceeds maximum %d", buf.Len(), mds)
	}

	// If we have a fixed size, and we haven't reached that size, write padding
	// bytes.
	if fs := ps.FixedSize; fs > 0 && buf.Len() < fs {
		remaining := fs - buf.Len()
		buf.Write(bytes.Repeat([]byte{0x00}, remaining))
	}

	// Send the packet through ds.
	if err := ds.SendDatagram(buf.Bytes()); err != nil {
		return err
	}

	// We successfully sent our packet. Increment our sequence number.
	ps.resetPacketBuffer(buf)
	ps.NextID++
	return nil
}
