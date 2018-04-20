// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package pixelpusher

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/danjacques/gopushpixels/support/dataio"

	"github.com/lunixbochs/struc"
	"github.com/pkg/errors"
)

const (
	// ListenPort is the port that PixelPusher devices tend to advertise for their
	// pixel data.
	//
	// It's probably not useful in practice, since all modern devices will specify
	// their port, but it's kept here for record.
	ListenPort uint16 = 5078

	// DefaultPort is the default PixelPusher device data port value to use when
	// the PixelPusher does not include a discovery header specifying this value.
	DefaultPort uint16 = 9798

	// MinAcceptableSoftwareRevision is the software revision prior to which a
	// warning to update software should be issued.
	//
	// This isn't prohibitive, and this packet supports earlier versions, but
	// other libraries use this as a signal so we're putting it here in case you
	// also want to.
	MinAcceptableSoftwareRevision uint16 = 121

	// LatestSoftwareRevision is the latest known software revision that this
	// package supports.
	LatestSoftwareRevision uint16 = 122
)

// PixelPusher device flags (software version > 108).
const (
	// PFlagProtected is the PFLAG_PROTECTED device flag.
	PFlagProtected = (1 << iota)
	// PFlagFixedSize is the PFLAG_FIXDEDSIZE device flag.
	//
	// It indicates that sent packets must be a fixed size (Photon).
	// This ends up being:
	//
	// 4 + ((1 + 3 * PixelsPerStrip) * min(StripsAttached, MaxStripsPerPacket));
	PFlagFixedSize
	// PFlagGlobalBrightness is the PFLAG_GLOBALBRIGHTNESS device flag.
	PFlagGlobalBrightness
	// PFlagStripBrightness is the PFLAG_STRIPBRIGHTNESS device flag.
	PFlagStripBrightness
	// PFlagMonochromeNotPacked is the PFLAG_MONOCHROME_NOT_PACKED device flag.
	PFlagMonochromeNotPacked
)

// Device is a device header extension for the DeviceType.
//
// /**
//  * uint8_t strips_attached;
//  * uint8_t max_strips_per_packet;
//  * uint16_t pixels_per_strip; // uint16_t used to make alignment work
//  * uint32_t update_period; // in microseconds
//  * uint32_t power_total; // in PWM units
//  * uint32_t delta_sequence; // difference between received and expected
//  * sequence numbers
//  * int32_t controller_ordinal;  // configured order number for controller
//  * int32_t group_ordinal;  // configured group number for this controller
//  * int16_t artnet_universe;
//  * int16_t artnet_channel;
//  * int16_t my_port;
//  */
type Device struct {
	DeviceHeader

	// The following headers *may* be present in the discovery packet, depending
	// on the software version.
	//
	// For DXXX, the header may be present if the SoftwareRevision device header
	// is >= XXX.
	//
	// If the headers are missing, these will be valid and contain default values.
	DeviceHeaderExt101
	DeviceHeaderExt109
	DeviceHeaderExt117

	Extra []byte
}

// DeviceHeader is the standard PixelPusher device header.
type DeviceHeader struct {
	StripsAttached     uint8
	MaxStripsPerPacket uint8
	PixelsPerStrip     uint16 `struc:",little"`
	UpdatePeriod       uint32 `struc:",little"`
	PowerTotal         uint32 `struc:",little"`
	DeltaSequence      uint32 `struc:",little"`
	ControllerOrdinal  int32  `struc:",little"`
	GroupOrdinal       int32  `struc:",little"`
	ArtNetUniverse     int16  `struc:",little"`
	ArtNetChannel      int16  `struc:",little"`
}

// DeviceHeaderExt101 is an optional extension of the Device header that can exist
// if the software revision is >= 101.
type DeviceHeaderExt101 struct {
	// MyPort is optionally present if the software version is > 100; otherwise,
	// it defaults to DefaultPort.
	MyPort uint16 `struc:",little"`

	// Inferred from Java code offsets.
	Pad2_3 []byte `struc:"[2]pad"`
}

// DeviceHeaderExt109 is an optional extension of the Device header that can exist
// if the software revision is >= 109.
//
// DeviceHeaderExt109 requires DeviceHeaderExt101 to be present.
type DeviceHeaderExt109 struct {
	// Flags for each strip. This must have len(StripsAttached).
	//
	// On the wire, a *minimum* of 8 entries must be present. The actual number
	// available will be min(8, StripsAttached).
	StripFlags []StripFlags
}

// DeviceHeaderExt117 is an optional extension of the Device header that can exist
// if the software revision is >= 117.
//
// DeviceHeaderExt117 requires DeviceHeaderExt101 to be present.
type DeviceHeaderExt117 struct {
	// Inferred from Java code offsets.
	Pad0_1 []byte `struc:"[2]pad"`

	// If software version is > 116, this may be followed by more PixelPusher
	// properties.
	PusherFlags uint32 `struc:",little"`
	Segments    uint32 `struc:",little"`
	PowerDomain uint32 `struc:",little"`
}

// ReadDevice reads a Device from r.
//
// ReadDevice will select what to read based on the presence of data in the
// header and the software version.
func ReadDevice(r io.Reader, swVersion uint16) (*Device, error) {
	var d Device

	// Called at perceived end of packet, to read the remainder into Extra.
	finish := func() (*Device, error) {
		v, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}

		d.Extra = v
		return &d, nil
	}

	// Read the standard header.
	if err := struc.Unpack(r, &d.DeviceHeader); err != nil {
		return nil, err
	}

	// Initialize default extension values.
	d.MyPort = DefaultPort
	d.StripFlags = make([]StripFlags, d.StripsAttached)

	// (Software Version >= 101)
	if swVersion < 101 {
		return finish()
	}

	var d101 DeviceHeaderExt101
	switch err := struc.Unpack(r, &d101); err {
	case nil:
		d.DeviceHeaderExt101 = d101
	case io.EOF:
		// The header was not present; return our defaults.
		return finish()
	default:
		return nil, errors.Wrap(err, "reading D101 extension header")
	}

	// (Software Version >= 109)
	if swVersion < 109 {
		return finish()
	}

	// Read our strip flag bytes. At least 8 flags will be present on the write,
	// regardless of how many devices are available, so we will read them all.
	numStripFlags := uint8(8)
	if numStripFlags < d.StripsAttached {
		numStripFlags = d.StripsAttached
	}
	stripFlags := make([]byte, numStripFlags)
	switch err := dataio.ReadFull(r, stripFlags); err {
	case nil:
		for i := range d.StripFlags {
			d.StripFlags[i] = StripFlags(stripFlags[i])
		}
	case io.EOF:
		// The header was not present; return our defaults.
		return finish()
	default:
		return nil, errors.Wrap(err, "reading D109 extension header")
	}

	// (Software Version >= 117)
	if swVersion < 117 {
		return finish()
	}

	var d117 DeviceHeaderExt117
	switch err := struc.Unpack(r, &d117); err {
	case nil:
		d.DeviceHeaderExt117 = d117
	case io.EOF:
		// The header was not present; return our defaults.
		return finish()
	default:
		return nil, errors.Wrap(err, "reading D117 extension header")
	}

	// (End of headers *whew*)
	return finish()
}

// Write writes this Device's header data to w.
//
// Parts of the header may not be written if swVersion doesn't allow it.
func (d *Device) Write(w io.Writer, swVersion uint16) error {
	dw := dataio.MakeWriter(w)

	// Write the common device header.
	if err := struc.Pack(dw, &d.DeviceHeader); err != nil {
		return err
	}

	// (Software Version >= 101)
	if swVersion < 101 {
		return nil
	}
	if err := struc.Pack(dw, &d.DeviceHeaderExt101); err != nil {
		return err
	}

	// (Software Version >= 109)
	if swVersion < 109 {
		return nil
	}

	// Write our strip flag bytes. We must write at least 8 bytes regardless of
	// how many devices we have attached, but may write more if we have more than
	// 8 attached devices.
	for _, sf := range d.StripFlags {
		if err := dw.WriteByte(byte(sf)); err != nil {
			return err
		}
	}
	for i := len(d.StripFlags); i < 8; i++ {
		// Write a padding byte to get us to 8.
		if err := dw.WriteByte(0x00); err != nil {
			return err
		}
	}

	// (Software Version >= 117)
	if swVersion < 117 {
		return nil
	}
	if err := struc.Pack(dw, &d.DeviceHeaderExt117); err != nil {
		return err
	}

	// (End of heders *whew*)
	return nil
}

func (d *Device) String() string {
	return fmt.Sprintf(
		"PixelPusher{strips_attached=%d, max_strips_per_packet=%d, pixels_per_strip=%d, "+
			"update_period=%s, power_total=%d, delta_sequence=%d, controller_ordinal=%d, "+
			"group_ordinal=%d, art_net_universe=%d, art_net_channel=%d, my_port=%d, "+
			"strip_flags={%v}, pusher_flags=0x%08x, segments=%d, power_domain=%d, extra=%v}",
		d.StripsAttached, d.MaxStripsPerPacket, d.PixelsPerStrip,
		d.UpdatePeriodDuration(), d.PowerTotal, d.DeltaSequence, d.ControllerOrdinal,
		d.GroupOrdinal, d.ArtNetUniverse, d.ArtNetChannel, d.MyPort,
		d.StripFlags, d.PusherFlags, d.Segments, d.PowerDomain, d.Extra)
}

// Clone creates a deep copy of d.
func (d *Device) Clone() *Device {
	clone := *d
	clone.StripFlags = append([]StripFlags(nil), clone.StripFlags...)
	return &clone
}

// UpdatePeriodDuration returns d's update period, experessed in microseconds,
// as a time.Duration.
func (d *Device) UpdatePeriodDuration() time.Duration {
	return time.Microsecond * time.Duration(d.UpdatePeriod)
}

// FixedSize returns the fixed-size packet value if one is set, or <=0 if this
// device does not require a fixed size (PFLAG_FIXEDSIZE).
func (d *Device) FixedSize() int {
	if 0 == d.PusherFlags&PFlagFixedSize {
		return 0
	}

	strips := int(d.MaxStripsPerPacket)
	if strips > int(d.StripsAttached) {
		strips = int(d.StripsAttached)
	}

	// [ID] + ForEachStrip(StripNumber + RGB)
	return 4 + ((1 + 3*int(d.PixelsPerStrip)) * strips)
}

// PacketReader returns a PacketReader configured for this Device.
func (d *Device) PacketReader() *PacketReader {
	return &PacketReader{
		PixelsPerStrip: int(d.PixelsPerStrip),
		StripFlags:     d.StripFlags,
	}
}

// PacketStream returns a PacketStream configured for this Device.
func (d *Device) PacketStream() *PacketStream {
	return &PacketStream{
		MaxStripsPerPacket: d.MaxStripsPerPacket,
		PixelsPerStrip:     d.PixelsPerStrip,
		FixedSize:          d.FixedSize(),
	}
}
