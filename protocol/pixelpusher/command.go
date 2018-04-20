// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	"bytes"
	"io"

	"github.com/danjacques/gopushpixels/support/dataio"

	"github.com/lunixbochs/struc"
	"github.com/pkg/errors"
)

// CommandMagic is a magic number that precedes PixelPusher commands.
var CommandMagic = []byte{
	0x40, 0x09, 0x2d, 0xa6, 0x15, 0xa5, 0xdd, 0xe5,
	0x6a, 0x9d, 0x4d, 0x5a, 0xcf, 0x09, 0xaf, 0x50,
}

// commandMagicLen is a constant equal to the length of CommandMagic.
const commandMagicLen = 16

// CommandID is an individual PixelPusher command value.
type CommandID uint8

/*  const uint8_t pp_command_magic[16] =
 *
 * #define COMMAND_RESET                0x01
 * #define COMMAND_GLOBALBRIGHTNESS_SET 0x02
 * #define COMMAND_WIFI_CONFIGURE       0x03
 * #define COMMAND_LED_CONFIGURE        0x04
 * #define COMMAND_STRIPBRIGHTNESS_SET  0x05
 */
const (
	CommandReset               CommandID = 0x01
	CommandGlobalBrightnessSet           = 0x02
	CommandWifiConfigure                 = 0x03
	CommandLEDConfigure                  = 0x04
	CommandStripBrightnessSet            = 0x05
)

// ColourOrder sets the pixel color order.
//
// typedef enum ColourOrder {
//   RGB=0, RBG=1, GBR=2, GRB=3, BGR=4, BRG=5} ColourOrder;
type ColourOrder uint64

const (
	// ColourOrderRGB is the colour order for RGB.
	ColourOrderRGB ColourOrder = 0
	// ColourOrderRBG is the colour order for RBG.
	ColourOrderRBG = 1
	// ColourOrderGBR is the colour order for GBR.
	ColourOrderGBR = 2
	// ColourOrderGRB is the colour order for GRB.
	ColourOrderGRB = 3
	// ColourOrderBGR is the colour order for BGR.
	ColourOrderBGR = 4
	// ColourOrderBRG is the colour order for BRG.
	ColourOrderBRG = 5
)

// Command is a general interface for the command.
type Command interface {
	// ID is the command ID for this command.
	ID() CommandID

	// WriteContentTo writes a command to w.
	//
	// WriteContentTo does not write the CommandMagic or the command byte.
	WriteContentTo(w io.Writer) error

	// LoadContentFrom reads a command's content from r.
	//
	// LoadContentFrom does not load the CommandMagic or the command byte.
	LoadContentFrom(r io.Reader) error
}

// ResetCommand issues a RESET command.
type ResetCommand struct{}

// ID implements Command.
func (*ResetCommand) ID() CommandID { return CommandReset }

// WriteContentTo implements Command.
func (cmd *ResetCommand) WriteContentTo(w io.Writer) error { return nil }

// LoadContentFrom implements Command.
//
// Note that RESET command has no contents so this will do nothing and succeed.
func (cmd *ResetCommand) LoadContentFrom(io.Reader) error { return nil }

// GlobalBrightnessSetCommand is a GLOBALBRIGHTNESS_SET command.
type GlobalBrightnessSetCommand struct {
	Parameter uint16 `struc:",little"`
}

// ID implements Command.
func (*GlobalBrightnessSetCommand) ID() CommandID { return CommandGlobalBrightnessSet }

// WriteContentTo implements Command.
func (cmd *GlobalBrightnessSetCommand) WriteContentTo(w io.Writer) error {
	return struc.Pack(w, cmd)
}

// LoadContentFrom implements Command.
//
// Note that RESET command has no contents so this will do nothing and succeed.
func (cmd *GlobalBrightnessSetCommand) LoadContentFrom(r io.Reader) error {
	return struc.Unpack(r, cmd)
}

// StripBrightnessSetCommand is a STRIPBRIGHTNESS_SET command.
type StripBrightnessSetCommand struct {
	StripNumber uint8
	Parameter   uint16 `struc:",little"`
}

// ID implements Command.
func (*StripBrightnessSetCommand) ID() CommandID { return CommandStripBrightnessSet }

// WriteContentTo implements Command.
func (cmd *StripBrightnessSetCommand) WriteContentTo(w io.Writer) error {
	return struc.Pack(w, cmd)
}

// LoadContentFrom implements Command.
//
// Note that RESET command has no contents so this will do nothing and succeed.
func (cmd *StripBrightnessSetCommand) LoadContentFrom(r io.Reader) error {
	return struc.Unpack(r, cmd)
}

// WiFiConfigureCommand is a WIFI_CONFIGURE command.
type WiFiConfigureCommand struct {
	SSID     string
	Key      string
	Security Security
}

// ID implements Command.
func (*WiFiConfigureCommand) ID() CommandID { return CommandWifiConfigure }

// WriteContentTo implements Command.
func (cmd *WiFiConfigureCommand) WriteContentTo(w io.Writer) error {
	dw := dataio.MakeWriter(w)

	// We can't just pack a struct, since this encoding includes NULL-terminated
	// strings.
	writeNULLTerminatedString := func(v string) error {
		if _, err := dw.Write([]byte(v)); err != nil {
			return err
		}
		return dw.WriteByte(0x00)
	}

	// Write SSID.
	if err := writeNULLTerminatedString(cmd.SSID); err != nil {
		return err
	}

	// Write key bytes.
	if err := writeNULLTerminatedString(cmd.Key); err != nil {
		return err
	}

	// Write security byte.
	return dw.WriteByte(byte(cmd.Security))
}

// LoadContentFrom implements Command.
//
// Note that RESET command has no contents so this will do nothing and succeed.
func (cmd *WiFiConfigureCommand) LoadContentFrom(r io.Reader) error {
	const stringBufferSize = 64
	dr := dataio.MakeReader(r)

	readNULLTerminatedString := func() (string, error) {
		// Read byte-by-byte until we hit a NULL.
		stringBytes := make([]byte, 0, stringBufferSize)
		for {
			switch v, err := dr.ReadByte(); {
			case err != nil:
				return "", err
			case v == 0x00:
				// NULL terminator, punt the string.
				return string(stringBytes), nil
			default:
				stringBytes = append(stringBytes, v)
			}
		}
	}

	// Read SSID.
	var err error
	if cmd.SSID, err = readNULLTerminatedString(); err != nil {
		return err
	}

	// Read key bytes.
	if cmd.Key, err = readNULLTerminatedString(); err != nil {
		return err
	}

	// Read security byte.
	security, err := dr.ReadByte()
	if err != nil {
		return err
	}
	cmd.Security = Security(security)
	return nil
}

// LEDConfigureCommand is a COMMAND_LED_CONFIGURE command.
type LEDConfigureCommand struct {
	NumStrips   uint32      `struc:",little"`
	StripLength uint32      `struc:",little"`
	StripType   uint64      `struc:",little"`
	ColourOrder ColourOrder `struc:",little"`
	Group       uint16      `struc:",little"`
	Controller  uint16      `struc:",little"`

	ArtNetUniverse uint16 `struc:",little"`
	ArtNetChannel  uint16 `struc:",little"`
}

// ID implements Command.
func (*LEDConfigureCommand) ID() CommandID { return CommandLEDConfigure }

// WriteContentTo implements Command.
func (cmd *LEDConfigureCommand) WriteContentTo(w io.Writer) error {
	return struc.Pack(w, cmd)
}

// LoadContentFrom implements Command.
//
// Note that RESET command has no contents so this will do nothing and succeed.
func (cmd *LEDConfigureCommand) LoadContentFrom(r io.Reader) error {
	return struc.Unpack(r, cmd)
}

// ReadCommand reads a Command data from r.
//
// If consumeMagic is true, ReadCommand will expect r to begin with the
// CommandMagic header, and will error if it doesn't.
//
// The user should use a buffered reader to support the various incremental
// reads that will need to be executed.
func ReadCommand(r io.Reader, consumeMagic bool) (Command, error) {
	dr := dataio.MakeReader(r)

	// Consume and assert the CommandMagic header, if requested.
	if consumeMagic {
		buf := make([]byte, len(CommandMagic))
		if err := dataio.ReadFull(dr, buf); err != nil {
			return nil, err
		}
		if !bytes.Equal(buf, CommandMagic) {
			return nil, errors.Errorf("command did not begin with magic: %v", buf)
		}
	}

	// Read the command byte.
	//
	// NOTE: we're tempted to read this along with header, but the reader should
	// be buffered, so it's not worth the complexity.
	cmdByte, err := dr.ReadByte()
	if err != nil {
		return nil, errors.Wrap(err, "while reading command byte")
	}

	// Identify the comamnd.
	var cmd Command
	switch CommandID(cmdByte) {
	case CommandReset:
		cmd = &ResetCommand{}
	case CommandGlobalBrightnessSet:
		cmd = &GlobalBrightnessSetCommand{}
	case CommandStripBrightnessSet:
		cmd = &StripBrightnessSetCommand{}
	case CommandWifiConfigure:
		cmd = &WiFiConfigureCommand{}
	case CommandLEDConfigure:
		cmd = &LEDConfigureCommand{}
	default:
		return nil, errors.Errorf("unknown command byte 0x%02x", cmdByte)
	}

	// Load the remainder of the command.
	if err := cmd.LoadContentFrom(dr); err != nil {
		return nil, errors.Wrapf(err, "failed to load command 0x%02x", cmdByte)
	}
	return cmd, nil
}

// WriteCommand writes a Command to w.
//
// If writeMagic is true, the CommandMagic header will be written at the
// beginning.
//
// The user should use a buffered writer to support the various incremental
// writes that will need to be executed.
func WriteCommand(cmd Command, w io.Writer, writeMagic bool) error {
	dw := dataio.MakeWriter(w)

	// Write the command magic header, if requested.
	if writeMagic {
		if _, err := dw.Write(CommandMagic); err != nil {
			return errors.Wrap(err, "failed to write command magic header")
		}
	}

	// Write the command byte.
	if err := dw.WriteByte(byte(cmd.ID())); err != nil {
		return errors.Wrap(err, "failed to write command byte")
	}

	if err := cmd.WriteContentTo(dw); err != nil {
		return errors.Wrap(err, "failed to write command content")
	}
	return nil
}
