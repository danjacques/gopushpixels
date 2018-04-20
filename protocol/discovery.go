// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package protocol

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"github.com/danjacques/gopushpixels/protocol/pixelpusher"

	"github.com/lunixbochs/struc"
	"github.com/pkg/errors"
)

const (
	// DefaultProtocolVersion is the default protocol version.
	//
	// A value of 1 was observed on modern PixelPusher devices.
	DefaultProtocolVersion = 1

	// DiscoveryUDPPort is the UDP port on which devices multicast announce over.
	DiscoveryUDPPort = 7331
)

// DeviceType is an enumeration representing the type of device in a
// DeviceHeader.
type DeviceType uint8

const (
	// EtherDreamDeviceType is the DeviceType for the EtherDream.
	EtherDreamDeviceType DeviceType = 0
	// LumiaBridgeDeviceType is the DeviceType for the LumiaBridge.
	LumiaBridgeDeviceType DeviceType = 1
	// PixelPusherDeviceType is the DeviceType for the PixelPusher.
	PixelPusherDeviceType DeviceType = 2
)

func (dt DeviceType) String() string {
	switch dt {
	case EtherDreamDeviceType:
		return "ETHERDREAM"
	case LumiaBridgeDeviceType:
		return "LUMIABRIDGE"
	case PixelPusherDeviceType:
		return "PIXELPUSHER"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", dt)
	}
}

// DeviceHeader is a discovery-related packet that represents a single
// device.
//
// /**
//  * Device Header format:
//  * uint8_t mac_address[6];
//  * uint8_t ip_address[4];
//  * uint8_t device_type;
//  * uint8_t protocol_version; // for the device, not the discovery
//  * uint16_t vendor_id;
//  * uint16_t product_id;
//  * uint16_t hw_revision;
//  * uint16_t sw_revision;
//  * uint32_t link_speed; // in bits per second
//  */
type DeviceHeader struct {
	MacAddress       [6]byte
	IPAddress        [4]byte
	DeviceType       DeviceType
	ProtocolVersion  uint8
	VendorID         uint16 `struc:",little"`
	ProductID        uint16 `struc:",little"`
	HardwareRevision uint16 `struc:",little"`
	SoftwareRevision uint16 `struc:",little"`

	// The link speed, in bits-per-second.
	LinkSpeed uint32 `struc:",little"`
}

// IP4Address returns a net.IP derived from the IPAddress field.
func (h *DeviceHeader) IP4Address() net.IP {
	return net.IPv4(h.IPAddress[0], h.IPAddress[1], h.IPAddress[2], h.IPAddress[3])
}

// SetIP4Address sets the IPAddress field from a net.IP.
func (h *DeviceHeader) SetIP4Address(ip net.IP) {
	ip4 := ip.To4()
	if ip4 == nil {
		panic("address is not an IPv4 address")
	}
	copy(h.IPAddress[:], ip4[:4])
}

// HardwareAddr returns the MacAddress field as a net.HardwareAddr.
func (h *DeviceHeader) HardwareAddr() net.HardwareAddr {
	return net.HardwareAddr(h.MacAddress[:])
}

// SetHardwareAddr sets the MacAddress value to addr.
func (h *DeviceHeader) SetHardwareAddr(addr net.HardwareAddr) {
	if len(addr) != 6 {
		panic("invalid hardware address length")
	}
	copy(h.MacAddress[:], addr)
}

// DiscoveryHeaders is the set of information contained in a discovery packet.
//
// NOTE: Since only PixelPusher devices are supported at the moment, this
// will always exist and be populated.
type DiscoveryHeaders struct {
	// DeviceHeader describes the generic device.
	DeviceHeader

	// PixelPusher describes the PixelPusher in detail.
	PixelPusher *pixelpusher.Device
}

// ParseDiscoveryHeaders parses discovery packet headers from provided byte
// array.
//
// If the device headers are invalid, or if all of the data was not consumed,
// an error will be returned.
func ParseDiscoveryHeaders(data []byte) (*DiscoveryHeaders, error) {
	var dh DiscoveryHeaders
	r := bytes.NewReader(data)

	// Read the device header.
	if err := struc.Unpack(r, &dh.DeviceHeader); err != nil {
		return nil, errors.Wrap(err, "could not unpack device header")
	}

	// Choose which device-specific header to read.
	switch dh.DeviceType {
	case PixelPusherDeviceType:
		var err error
		if dh.PixelPusher, err = pixelpusher.ReadDevice(r, dh.SoftwareRevision); err != nil {
			return nil, errors.Wrap(err, "could not unpack PixelPusher data block")
		}

	default:
		return nil, errors.Errorf("unsupported device type (%d)", dh.DeviceType)
	}

	return &dh, nil
}

// WritePacket writes a discovery packet to w.
func (dh *DiscoveryHeaders) WritePacket(w io.Writer) error {
	if err := struc.Pack(w, &dh.DeviceHeader); err != nil {
		return err
	}

	// Write the device-specific header.
	switch {
	case dh.PixelPusher != nil:
		return dh.PixelPusher.Write(w, dh.SoftwareRevision)
	}
	return nil
}

func (dh *DiscoveryHeaders) String() string {
	var impl interface{}
	switch {
	case dh.PixelPusher != nil:
		impl = dh.PixelPusher
	}

	return fmt.Sprintf(
		"Device{mac_address=%s, ip_address=%s, device_type=%s, protocol_version=%d, "+
			"vendor_id=0x%x, product_id=0x%x, hardware_revision=%d, software_revision=%d, "+
			"link_speed=%d, impl=%s}",
		dh.HardwareAddr(), dh.IP4Address(), dh.DeviceType, dh.ProtocolVersion,
		dh.VendorID, dh.ProductID, dh.HardwareRevision, dh.SoftwareRevision,
		dh.LinkSpeed, impl)
}

// Clone creates a deep copy of dh.
func (dh *DiscoveryHeaders) Clone() *DiscoveryHeaders {
	clone := *dh

	switch {
	case clone.PixelPusher != nil:
		clone.PixelPusher = clone.PixelPusher.Clone()
	}

	return &clone
}

// Addr returns this device's network address, as described by its headers.
func (dh *DiscoveryHeaders) Addr() net.Addr {
	switch dh.DeviceType {
	case PixelPusherDeviceType:
		return &net.UDPAddr{
			IP:   dh.IP4Address(),
			Port: int(dh.PixelPusher.MyPort),
		}
	default:
		return &net.IPAddr{
			IP: dh.IP4Address(),
		}
	}
}

// NumStrips returns the number of strips represented by dh.
//
// IF the number could not be determined, NumLEDs will return 0.
func (dh *DiscoveryHeaders) NumStrips() int {
	switch {
	case dh.PixelPusher != nil:
		return int(dh.PixelPusher.StripsAttached)
	default:
		return 0
	}
}

// NumPixels returns the total number of pixels represented by dh.
//
// IF the number could not be determined, NumLEDs will return 0.
func (dh *DiscoveryHeaders) NumPixels() int {
	switch {
	case dh.PixelPusher != nil:
		return int(dh.PixelPusher.PixelsPerStrip) * int(dh.PixelPusher.StripsAttached)
	default:
		return 0
	}
}

// PacketReader creates a configured PacketReader instance for this device.
func (dh *DiscoveryHeaders) PacketReader() (*PacketReader, error) {
	// Choose which device-specific header to read.
	switch dh.DeviceType {
	case PixelPusherDeviceType:
		return &PacketReader{
			PixelPusher: dh.PixelPusher.PacketReader(),
		}, nil

	default:
		return nil, errors.Errorf("packet reader is not supported for device (%s)", dh.DeviceType)
	}
}

// PacketStream creates a configured PacketStream instance for this device.
func (dh *DiscoveryHeaders) PacketStream() (*PacketStream, error) {
	// Choose which device-specific header to read.
	switch dh.DeviceType {
	case PixelPusherDeviceType:
		return &PacketStream{
			PixelPusher: dh.PixelPusher.PacketStream(),
		}, nil

	default:
		return nil, errors.Errorf("packet stream is not supported for device (%s)", dh.DeviceType)
	}
}
