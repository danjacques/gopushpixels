package streamfile

import (
	"os"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol/pixelpusher"

	"github.com/pkg/errors"
)

// DeviceOrdinal returns a device ordinal for this device.
//
// Is the device has an invalid ordinal, Ordinal will return an invalid ordinal.
func (s *Device) DeviceOrdinal() device.Ordinal {
	if o := s.Ordinal; o != nil {
		return device.Ordinal{
			Group:      int(o.Group),
			Controller: int(o.Controller),
		}
	}
	return device.InvalidOrdinal()
}

// StripFlags returns a flags representation for each strip in this Device.
func (s *Device) StripFlags() []pixelpusher.StripFlags {
	flags := make([]pixelpusher.StripFlags, len(s.Strip))
	for i, s := range s.Strip {
		flags[i] = s.StripFlags()
	}
	return flags
}

// StripFlags returns the StripFlags equivalent data for this Strip.
func (s *Device_Strip) StripFlags() pixelpusher.StripFlags {
	var sf pixelpusher.StripFlags
	sf.SetRGBOW(s.PixelType == Device_Strip_RGBOW)
	return sf
}

// Validate validates that path is a valid stream file.
func Validate(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return errors.New("is not a directory")
	}

	var md Metadata
	if err := LoadMetadata(path, &md); err != nil {
		return errors.Wrap(err, "could not load metadata")
	}

	return nil
}

// Delete deletes the stream file at the specified path.
func Delete(path string) error { return os.RemoveAll(path) }
