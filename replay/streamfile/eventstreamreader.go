package streamfile

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/danjacques/gopushpixels/support/protostream"

	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
)

// EventStreamReader reads events from a stream.
//
// EventStreamReader must be instantiated using MakeEventStreamReader. After
// instantiation, EventStreamReader can be modified to control its behavior.
type EventStreamReader struct {
	// path is a path to the event stream directory.
	path string

	// rsr is a buffered reader for this stream.
	rsr rawStreamReader

	// baseFile is the base file that is being read.
	baseFile *os.File
	// index is the index of the current event file.
	nextIndex int

	// dec is a persistent decoder instance.
	dec protostream.Decoder

	// md is the metadata for this directory, loaded from its metadata file.
	md Metadata

	// lastEventOffset was the offset of the latest event in the stream.
	lastEventOffset time.Duration
	// cumulativeOffset is the offset from preceding files.
	cumulatoiveOffset time.Duration

	// efi is the current event file being read, or nil if none is being read.
	efi *Metadata_EventFile
	// efiDeviceMap is a map of efi's internal indicers of the current event file
	// to their larger respective Devices.
	//
	// This is re-initialized each time a new stream file is read.
	efiDeviceMap map[int64]*Device
}

// MakeEventStreamReader instantiates a new EventStreamReader.
func MakeEventStreamReader(path string) (*EventStreamReader, error) {
	esr := EventStreamReader{
		path: path,
	}

	// Load the metadata from this path.
	if err := LoadMetadata(path, &esr.md); err != nil {
		return nil, errors.Wrap(err, "loading metadata")
	}

	// Reset our reader to the first file.
	if err := esr.Reset(); err != nil {
		return nil, err
	}

	return &esr, nil
}

// Reset clears state and causes the reader to reload the file.
//
// Reset keeps the underlying file open.
func (esr *EventStreamReader) Reset() error {
	// IF we have a stream open, close it.
	if esr.baseFile != nil {
		if err := esr.baseFile.Close(); err != nil {
			return err
		}
		esr.baseFile = nil
	}

	esr.efi = nil
	esr.nextIndex = 0
	esr.lastEventOffset = 0
	esr.cumulatoiveOffset = 0
	return nil
}

// Path returns the path of the stream file.
//
// Path will return the path to the base directory.
func (esr *EventStreamReader) Path() string { return esr.path }

// Position returns the offset of the latest read event.
func (esr *EventStreamReader) Position() time.Duration {
	return esr.cumulatoiveOffset + esr.lastEventOffset
}

// Duration returns the duration, loaded from the metadata duration field.
//
// If the duration field is missing or invalid, a duration of 0 will be
// returned.
func (esr *EventStreamReader) Duration() time.Duration {
	d := esr.md.Duration
	if d == nil {
		return 0
	}
	if v, err := ptypes.Duration(d); err == nil {
		return v
	}
	return 0
}

// Close closes the reader, freeing its underlying file.
func (esr *EventStreamReader) Close() error {
	return esr.closeBaseFile()
}

func (esr *EventStreamReader) closeBaseFile() error {
	if esr.baseFile == nil {
		return nil
	}

	if err := esr.baseFile.Close(); err != nil {
		return err
	}
	esr.baseFile, esr.efi = nil, nil
	return nil
}

// Metadata returns the metadata block for this file.
func (esr *EventStreamReader) Metadata() *Metadata { return &esr.md }

// GetDevice is a utility method to retrieve a device by index or return nil if
// the index is out of bounds.
//
// index is taken in context of the current file being read.
func (esr *EventStreamReader) GetDevice(index int64) *Device {
	if esr.efi == nil || index < 0 {
		return nil
	}

	// Resolve the internal index to the Devices list index.
	if index >= int64(len(esr.efi.DeviceMapping)) {
		return nil
	}
	index = esr.efi.DeviceMapping[index]

	// Resolve the device.
	if index >= int64(len(esr.md.Devices)) {
		return nil
	}
	return esr.md.Devices[index]
}

// ReadEvent returns the next event in the stream.
//
// Once ReadEvent returns successfully, the state of the stream can be further
// interrogated until the next ReadEvent call is made.
//
// If the end of the stream is encountered, ReadEvent will return io.EOF.
func (esr *EventStreamReader) ReadEvent() (*Event, error) {
	// Cycle through files until we hit end of stream or get an event.
	var e Event
	for {
		if err := esr.maybeBeginNextFile(); err != nil {
			// Can be EOF, indicating a true end of stream.
			return nil, err
		}

		// Read an event from the current file.
		switch _, err := esr.dec.Read(esr.rsr, &e); err {
		case nil:
			// Successfully read the event; advance our latest offset.
			//
			// If it fails, ignore it.
			if v, err := ptypes.Duration(e.Offset); err == nil && v > esr.lastEventOffset {
				esr.lastEventOffset = v
			}
			return &e, nil

		case io.EOF:
			// Hit end of this file, repeat the loop, possibly enqueueing the next
			// file.
			//
			// Add the total offset of the file that we just finished to our
			// cumulative offset.
			if d, err := ptypes.Duration(esr.efi.Duration); err == nil {
				esr.cumulatoiveOffset += d
			}
			if err := esr.closeBaseFile(); err != nil {
				return nil, err
			}
			continue

		default:
			return nil, err
		}
	}
}

// ResolveDeviceForIndex resolves the Device event stream record for the
// specified stream-internal device index.
//
// ResolveDeviceForIndex is only valid until the next ReadEvent is called.
func (esr *EventStreamReader) ResolveDeviceForIndex(index int64) *Device {
	return esr.efiDeviceMap[index]
}

// beginNextFile will open the next file in the stream.
//
// If there are no more files in the stream, beginNextFile will return io.EOF.
func (esr *EventStreamReader) maybeBeginNextFile() error {
	// If we already have a file open, use it.
	if esr.baseFile != nil {
		return nil
	}

	// If we've exceeded our event file length, and there are no more files,
	// return io.EOF.
	if esr.nextIndex >= len(esr.md.EventFileInfo) {
		// No more files in the stream, so we've hit end of file!
		esr.efi, esr.efiDeviceMap = nil, nil
		return io.EOF
	}

	// Reset our per-file state.
	esr.lastEventOffset = 0

	efi := esr.md.EventFileInfo[esr.nextIndex]
	nextFile := filepath.Join(esr.path, efi.Name)
	fd, err := os.Open(nextFile)
	if err != nil {
		return errors.Wrapf(err, "loading file #%d: %s", esr.nextIndex, nextFile)
	}
	esr.baseFile, esr.nextIndex = fd, esr.nextIndex+1

	if err := esr.rsr.reset(esr.baseFile, efi.Compression); err != nil {
		return errors.Wrap(err, "resetting reader")
	}

	// Commit to this event file.
	esr.efi = efi

	// Build the event file's device map against the current Metadata.
	esr.efiDeviceMap = make(map[int64]*Device, len(efi.DeviceMapping))
	for i, deviceIndex := range efi.DeviceMapping {
		if deviceIndex < 0 || deviceIndex >= int64(len(esr.md.Devices)) {
			continue
		}
		esr.efiDeviceMap[int64(i)] = esr.md.Devices[deviceIndex]
	}

	return nil
}
