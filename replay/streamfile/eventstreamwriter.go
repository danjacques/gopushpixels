// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"os"
	"path/filepath"
	"time"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/protostream"
	"github.com/danjacques/gopushpixels/support/stagingdir"

	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
)

const (
	// eventFileExt is the extension used for event stream binary files.
	eventFileExt = ".protostream"

	// Choose our event file name. Currently, we only use one event file.
	eventFileName = "events" + eventFileExt
)

// EventStreamWriter reads packets from a stream.
type EventStreamWriter struct {
	*EventStreamConfig

	// firstWrite is true if this is the first packet write.
	firstWrite bool

	// packetW is the writer used for streams.
	packetW *rawStreamWriter
	// stagingDir is the staging directory that we're using.
	stagingDir *stagingdir.D

	// destPath is the final destination path.
	destPath string

	// enc is a persistent stream encoder instance.
	enc protostream.Encoder

	mb *MetadataBuilder

	startTime time.Time
}

// MakeEventStreamWriter creates a EventStreamWriter instance.
func (cfg *EventStreamConfig) MakeEventStreamWriter(path, displayName string) (*EventStreamWriter, error) {
	// Create a metadata builder.
	mb, err := cfg.NewMetadataBuilder(displayName)
	if err != nil {
		return nil, err
	}

	// Create a temporary directory to stage our files in.
	stagingDir, err := stagingdir.New(cfg.TempDir, filepath.Base(path))
	if err != nil {
		return nil, errors.Wrap(err, "creating temporary directory")
	}
	defer func() {
		// Cleanup if we failed to complete our creation.
		if stagingDir != nil {
			_ = stagingDir.Destroy()
		}
	}()

	// Create our stream file within stagingDir.
	eventFilePath := stagingDir.Path(eventFileName)
	fd, err := os.Create(eventFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "creating event file")
	}
	packetW := newRawStreamWriter(fd)
	defer func() {
		if packetW != nil {
			_ = packetW.Close()
		}
	}()

	// Register this file with our metadata.
	mb.AddEventFile(eventFileName, cfg.WriterCompression)

	// Enable our file-level settings.
	if err := packetW.beginCompression(cfg.WriterCompression, cfg.WriterCompressionLevel); err != nil {
		return nil, errors.Wrap(err, "enabling packet writer compression")
	}

	// Set up a writer for this file. It will take possession of "fd".
	//
	// Note that this will write a TEMPORARY intermediate file that will be
	// converted to the final, readable file on Close.
	esw := EventStreamWriter{
		EventStreamConfig: cfg,
		firstWrite:        true,
		packetW:           packetW,
		stagingDir:        stagingDir,
		destPath:          path,
		mb:                mb,
	}

	stagingDir, packetW = nil, nil // Don't delete, owned by esw.
	return &esw, nil
}

// Path returns the path of the stream file being written.
//
// Path will return the path to the destination directory, not the intermediate
// directory used to construct it.
func (esw *EventStreamWriter) Path() string { return esw.destPath }

// NumEvents is the number of events that have been recorded so far.
func (esw *EventStreamWriter) NumEvents() int64 { return esw.mb.NumEvents() }

// NumBytes is the number of bytes that have been recorded so far.
func (esw *EventStreamWriter) NumBytes() int64 { return esw.mb.NumBytes() }

// Duration is the total duration fo the recording so far.
func (esw *EventStreamWriter) Duration() time.Duration { return esw.mb.Offset() }

// WritePacket writes a protocol packet event to the stream.
func (esw *EventStreamWriter) WritePacket(d device.D, pkt *protocol.Packet) error {
	// Determine our device index (possibly registering it in the process).
	deviceIndex := esw.observeDeviceAndGetIndex(d)

	// Encode this event.
	epkts, err := EncodePacket(deviceIndex, pkt)
	if err != nil {
		return err
	}

	// If it encoded into no packets, there's nothing to write.
	if len(epkts) == 0 {
		return nil
	}

	// Common Event. We will reuse this basic scaffolding, changing out each data
	// as we write.
	packetContainer := &Event_Packet_{}
	e := Event{
		Data: packetContainer,
	}

	// Calculate the event delta.
	now := esw.now()
	var offset time.Duration
	if esw.startTime.IsZero() {
		esw.startTime = now
	} else {
		offset = now.Sub(esw.startTime)
		if offset > 0 {
			e.Offset = ptypes.DurationProto(offset)
		}
	}

	// Write the event to the Writer.
	total := int64(0)
	for _, epkt := range epkts {
		packetContainer.Packet = epkt
		amt, err := esw.enc.Write(esw.packetW, &e)
		if err != nil {
			return err
		}

		total += int64(amt)
	}

	esw.mb.RecordEvent(total, offset)
	return nil
}

// Close closes the StreamWriter, finalizing the stream and releasing its
// resources.
func (esw *EventStreamWriter) Close() error {
	// Always delete our staging directory. If it's been committed, this will be
	// a no-op.
	defer func() {
		_ = esw.stagingDir.Destroy()
	}()

	// Close our packet stream.
	if err := esw.packetW.Close(); err != nil {
		return err
	}

	// Write our finalized file.
	//
	// If we have no packets, then don't write any finished file. Our temporary
	// one will be deleted. No point in wasting space on nothing.
	if esw.mb.NumEvents() == 0 {
		return nil
	}

	if err := esw.buildFinalFile(); err != nil {
		return errors.Wrap(err, "building final file")
	}

	return nil
}

func (esw *EventStreamWriter) observeDeviceAndGetIndex(d device.D) int64 {
	id := d.ID()
	return esw.mb.GetDeviceInternalIndex(id, func() *Device {
		// Add a new device to our Metadata block.
		device := Device{
			Id: id,
		}

		// Set the device's ordinal, presering ordinal validity.
		o := d.Ordinal()
		if o.IsValid() {
			device.Ordinal = &Device_Ordinal{
				Group:      int32(o.Group),
				Controller: int32(o.Controller),
			}
		}

		dh := d.DiscoveryHeaders()
		if pp := dh.PixelPusher; pp != nil {
			device.PixelsPerStrip = int64(pp.PixelsPerStrip)
			device.Strip = make([]*Device_Strip, len(pp.StripFlags))
			for i, sf := range pp.StripFlags {
				strip := Device_Strip{
					PixelType: Device_Strip_RGB,
				}
				if sf.IsRGBOW() {
					strip.PixelType = Device_Strip_RGBOW
				}
				device.Strip[i] = &strip
			}
		}
		return &device
	})
}

// buildFinalFile operates within the temporary directory until the end, when
// it has constructed the final file directory and moves it to its intended
// destination.
func (esw *EventStreamWriter) buildFinalFile() error {
	// Write our metadata file.
	if err := esw.mb.Write(esw.stagingDir.Path(metadataFileName)); err != nil {
		return errors.Wrap(err, "writing metadata file")
	}

	// Move the final directory into place (atomic).
	if err := esw.stagingDir.Commit(esw.destPath); err != nil {
		return errors.Wrap(err, "committing staging dir")
	}

	// We've successfully buit the final file!
	return nil
}
