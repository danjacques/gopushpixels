// Code generated by protoc-gen-go. DO NOT EDIT.
// source: event.proto

package streamfile

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import google_protobuf "github.com/golang/protobuf/ptypes/duration"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// Packet is an individual packet.
type Event struct {
	// The offset, from 0, of this packet.
	Offset *google_protobuf.Duration `protobuf:"bytes,1,opt,name=offset" json:"offset,omitempty"`
	// Event is the content of the event.
	//
	// Types that are valid to be assigned to Data:
	//	*Event_Packet_
	Data isEvent_Data `protobuf_oneof:"data"`
}

func (m *Event) Reset()                    { *m = Event{} }
func (m *Event) String() string            { return proto.CompactTextString(m) }
func (*Event) ProtoMessage()               {}
func (*Event) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{0} }

type isEvent_Data interface {
	isEvent_Data()
}

type Event_Packet_ struct {
	Packet *Event_Packet `protobuf:"bytes,2,opt,name=packet,oneof"`
}

func (*Event_Packet_) isEvent_Data() {}

func (m *Event) GetData() isEvent_Data {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *Event) GetOffset() *google_protobuf.Duration {
	if m != nil {
		return m.Offset
	}
	return nil
}

func (m *Event) GetPacket() *Event_Packet {
	if x, ok := m.GetData().(*Event_Packet_); ok {
		return x.Packet
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*Event) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _Event_OneofMarshaler, _Event_OneofUnmarshaler, _Event_OneofSizer, []interface{}{
		(*Event_Packet_)(nil),
	}
}

func _Event_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*Event)
	// data
	switch x := m.Data.(type) {
	case *Event_Packet_:
		b.EncodeVarint(2<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Packet); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("Event.Data has unexpected type %T", x)
	}
	return nil
}

func _Event_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*Event)
	switch tag {
	case 2: // data.packet
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(Event_Packet)
		err := b.DecodeMessage(msg)
		m.Data = &Event_Packet_{msg}
		return true, err
	default:
		return false, nil
	}
}

func _Event_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*Event)
	// data
	switch x := m.Data.(type) {
	case *Event_Packet_:
		s := proto.Size(x.Packet)
		n += proto.SizeVarint(2<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

// A single encoded packet, sent to a device.
type Event_Packet struct {
	// The index of the device to use.
	//
	// Mapping devices to indexes occurs externally.
	Device int64 `protobuf:"varint,1,opt,name=device" json:"device,omitempty"`
	// The packet contents.
	//
	// Types that are valid to be assigned to Contents:
	//	*Event_Packet_PixelpusherPixels
	Contents isEvent_Packet_Contents `protobuf_oneof:"contents"`
}

func (m *Event_Packet) Reset()                    { *m = Event_Packet{} }
func (m *Event_Packet) String() string            { return proto.CompactTextString(m) }
func (*Event_Packet) ProtoMessage()               {}
func (*Event_Packet) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{0, 0} }

type isEvent_Packet_Contents interface {
	isEvent_Packet_Contents()
}

type Event_Packet_PixelpusherPixels struct {
	PixelpusherPixels *PixelPusherPixels `protobuf:"bytes,3,opt,name=pixelpusher_pixels,json=pixelpusherPixels,oneof"`
}

func (*Event_Packet_PixelpusherPixels) isEvent_Packet_Contents() {}

func (m *Event_Packet) GetContents() isEvent_Packet_Contents {
	if m != nil {
		return m.Contents
	}
	return nil
}

func (m *Event_Packet) GetDevice() int64 {
	if m != nil {
		return m.Device
	}
	return 0
}

func (m *Event_Packet) GetPixelpusherPixels() *PixelPusherPixels {
	if x, ok := m.GetContents().(*Event_Packet_PixelpusherPixels); ok {
		return x.PixelpusherPixels
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*Event_Packet) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _Event_Packet_OneofMarshaler, _Event_Packet_OneofUnmarshaler, _Event_Packet_OneofSizer, []interface{}{
		(*Event_Packet_PixelpusherPixels)(nil),
	}
}

func _Event_Packet_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*Event_Packet)
	// contents
	switch x := m.Contents.(type) {
	case *Event_Packet_PixelpusherPixels:
		b.EncodeVarint(3<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.PixelpusherPixels); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("Event_Packet.Contents has unexpected type %T", x)
	}
	return nil
}

func _Event_Packet_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*Event_Packet)
	switch tag {
	case 3: // contents.pixelpusher_pixels
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(PixelPusherPixels)
		err := b.DecodeMessage(msg)
		m.Contents = &Event_Packet_PixelpusherPixels{msg}
		return true, err
	default:
		return false, nil
	}
}

func _Event_Packet_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*Event_Packet)
	// contents
	switch x := m.Contents.(type) {
	case *Event_Packet_PixelpusherPixels:
		s := proto.Size(x.PixelpusherPixels)
		n += proto.SizeVarint(3<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

// PixelPusherPixels is a set-pixels message for a single strip on a device.
//
// The actual pixel bytes are the raw pixel bitmap for the strip. Their meaning
// depends on the strip's pixel encoding.
type PixelPusherPixels struct {
	// The strip that the pixels are being sent to.
	StripNumber int32 `protobuf:"varint,1,opt,name=strip_number,json=stripNumber" json:"strip_number,omitempty"`
	// The raw pixel data for this strip.
	PixelData []byte `protobuf:"bytes,2,opt,name=pixel_data,json=pixelData,proto3" json:"pixel_data,omitempty"`
}

func (m *PixelPusherPixels) Reset()                    { *m = PixelPusherPixels{} }
func (m *PixelPusherPixels) String() string            { return proto.CompactTextString(m) }
func (*PixelPusherPixels) ProtoMessage()               {}
func (*PixelPusherPixels) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{1} }

func (m *PixelPusherPixels) GetStripNumber() int32 {
	if m != nil {
		return m.StripNumber
	}
	return 0
}

func (m *PixelPusherPixels) GetPixelData() []byte {
	if m != nil {
		return m.PixelData
	}
	return nil
}

func init() {
	proto.RegisterType((*Event)(nil), "streamfile.Event")
	proto.RegisterType((*Event_Packet)(nil), "streamfile.Event.Packet")
	proto.RegisterType((*PixelPusherPixels)(nil), "streamfile.PixelPusherPixels")
}

func init() { proto.RegisterFile("event.proto", fileDescriptor1) }

var fileDescriptor1 = []byte{
	// 273 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x90, 0x51, 0x4b, 0xfb, 0x30,
	0x14, 0xc5, 0xb7, 0xff, 0xfe, 0x0b, 0x7a, 0xbb, 0x97, 0xe5, 0x41, 0xea, 0x60, 0xa2, 0x7b, 0xf2,
	0x29, 0xe2, 0xfc, 0x06, 0x63, 0xc2, 0x9e, 0x46, 0x09, 0xf8, 0x5c, 0xd2, 0xf6, 0x76, 0x16, 0xbb,
	0x26, 0x24, 0xb7, 0x43, 0xc1, 0x8f, 0xed, 0x07, 0x90, 0xde, 0x16, 0x1d, 0xf8, 0x76, 0x72, 0xee,
	0x39, 0xf7, 0x97, 0x04, 0x22, 0x3c, 0x61, 0x43, 0xca, 0x79, 0x4b, 0x56, 0x42, 0x20, 0x8f, 0xe6,
	0x58, 0x56, 0x35, 0x2e, 0x6e, 0x1c, 0x7d, 0x38, 0x0c, 0x0f, 0x45, 0xeb, 0x0d, 0x55, 0xb6, 0xf9,
	0x11, 0x7d, 0x76, 0xf5, 0x35, 0x86, 0xe9, 0x73, 0xd7, 0x95, 0x8f, 0x20, 0x6c, 0x59, 0x06, 0xa4,
	0x78, 0x7c, 0x3b, 0xbe, 0x8f, 0xd6, 0xd7, 0xea, 0x60, 0xed, 0xa1, 0xc6, 0x3e, 0x98, 0xb5, 0xa5,
	0xda, 0x0e, 0x55, 0x3d, 0x04, 0xe5, 0x1a, 0x84, 0x33, 0xf9, 0x1b, 0x52, 0xfc, 0x8f, 0x2b, 0xb1,
	0xfa, 0x25, 0x2b, 0xde, 0xaa, 0x12, 0x9e, 0xef, 0x46, 0x7a, 0x48, 0x2e, 0x3e, 0x41, 0xf4, 0x9e,
	0xbc, 0x02, 0x51, 0xe0, 0xa9, 0xca, 0x91, 0x81, 0x13, 0x3d, 0x9c, 0xe4, 0x1e, 0xa4, 0xab, 0xde,
	0xb1, 0x76, 0x6d, 0x78, 0x45, 0x9f, 0xb2, 0x0e, 0xf1, 0x84, 0x09, 0xcb, 0x73, 0x42, 0xd2, 0x4d,
	0x12, 0x4e, 0xb1, 0x0c, 0xbb, 0x91, 0x9e, 0x9f, 0x55, 0x7b, 0x73, 0x03, 0x70, 0x91, 0xdb, 0x86,
	0xb0, 0xa1, 0xb0, 0x11, 0xf0, 0xbf, 0x30, 0x64, 0x56, 0x2f, 0x30, 0xff, 0xd3, 0x96, 0x77, 0x30,
	0x0b, 0xe4, 0x2b, 0x97, 0x36, 0xed, 0x31, 0x43, 0xcf, 0xd7, 0x9a, 0xea, 0x88, 0xbd, 0x3d, 0x5b,
	0x72, 0x09, 0xc0, 0x80, 0xb4, 0xdb, 0xc2, 0xaf, 0x9e, 0xe9, 0x4b, 0x76, 0xb6, 0x86, 0x4c, 0x26,
	0xf8, 0xaf, 0x9e, 0xbe, 0x03, 0x00, 0x00, 0xff, 0xff, 0x54, 0x5c, 0x5e, 0x74, 0x8f, 0x01, 0x00,
	0x00,
}
