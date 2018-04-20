// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
)

const (
	// metadataVersion is a compatibility version value for our metadata file.
	metadataVersion = "v1"
	// metadataMinorVersion is the current metadata minor version.
	metadataMinorVersion = 1

	// metadataFileName is the name of the metadata file.
	metadataFileName = "metadata." + metadataVersion + ".proto.text"

	// oldMetadataFileName is the name of the metadata file.
	// TODO: Deprecate me.
	oldMetadataFileName = "metadata." + metadataVersion + ".proto.bin"
)

// DeviceForInternalIndex returns the device with the specified name, or nil if
// no such device exists.
func (efi *Metadata_EventFile) DeviceForInternalIndex(index int64, md *Metadata) *Device {
	if index < 0 {
		return nil
	}

	// Resolve internal index to md device index.
	if index >= int64(len(efi.DeviceMapping)) {
		return nil
	}
	index = efi.DeviceMapping[index]

	// Resolve to Devices list.
	if index >= int64(len(md.Devices)) {
		return nil
	}
	return md.Devices[index]
}

// LoadMetadata loads information block from the named file.
//
// If the loaded Metadata file is not completely up-to-date, LoadMetadata will
// attempt to patch it.
func LoadMetadata(path string, md *Metadata) error {
	data, err := ioutil.ReadFile(filepath.Join(path, metadataFileName))

	// If the metadata filename is missing, try the old metadata filename.
	// TODO: Deprecate me.
	if err != nil && os.IsNotExist(err) {
		data, err = ioutil.ReadFile(filepath.Join(path, oldMetadataFileName))
	}

	if err != nil {
		return err
	}

	if err := proto.UnmarshalText(string(data), md); err != nil {
		return err
	}

	// Migrate the metadata to the current version.
	if err := migrateMetadata(md); err != nil {
		return errors.Wrap(err, "migrating metadata")
	}

	return nil
}

// LoadMetadataAndSize loads the metadata and total data file size for the
// specific path.
func LoadMetadataAndSize(path string) (*Metadata, int64, error) {
	var md Metadata
	if err := LoadMetadata(path, &md); err != nil {
		return nil, 0, err
	}

	size := int64(0)
	for _, efi := range md.EventFileInfo {
		path := filepath.Join(path, efi.Name)
		st, err := os.Stat(path)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "stat event file %s", path)
		}
		size += st.Size()
	}
	return &md, size, nil
}

// MetadataBuilder constructs a Metadata protobuf.
type MetadataBuilder struct {
	deviceIndexMap     map[string]int64
	cumulativeDuration time.Duration

	// If there is a current file, these track its information.
	currentFileInfo       *Metadata_EventFile
	currentFileOffset     time.Duration
	currentDeviceIndexMap map[string]int64

	meta Metadata
}

// NewMetadataBuilder constructs a new metadata builder instance.
func (cfg *EventStreamConfig) NewMetadataBuilder(name string) (*MetadataBuilder, error) {
	created, err := ptypes.TimestampProto(cfg.now())
	if err != nil {
		return nil, errors.Wrap(err, "creating timestamp proto")
	}

	return &MetadataBuilder{
		meta: Metadata{
			Version: Metadata_V_1,
			Minor:   metadataMinorVersion,
			Name:    name,
			Created: created,
		},
	}, nil
}

// GetDeviceInternalIndex returns the current event stream file's internal index
// for the device with the specified ID.
//
// If this device is not registered with the event stream file, it will register
// it, potentially generating and registering it with the larger Metadata
// Devices list if needed.
func (mb *MetadataBuilder) GetDeviceInternalIndex(id string, gen func() *Device) int64 {
	// If the device is already registered with the local event, return it.
	if fileIndex, ok := mb.currentDeviceIndexMap[id]; ok {
		return fileIndex
	}

	// Get the device index for "id", potentially generating/registering it if it
	// is not registered in Metadata.Devices yet.
	deviceIndex := mb.maybeRegisterDevice(id, gen)

	// Register this device with our event file.
	fileIndex := int64(len(mb.currentFileInfo.DeviceMapping))
	mb.currentFileInfo.DeviceMapping = append(mb.currentFileInfo.DeviceMapping, deviceIndex)

	// Add it to our currentDeviceIndexMap for speedy lookup.
	if mb.currentDeviceIndexMap == nil {
		mb.currentDeviceIndexMap = make(map[string]int64)
	}
	mb.currentDeviceIndexMap[id] = fileIndex

	return fileIndex
}

// maybeRegisterDevice returns the Metadata.Devices list index for the device
// with the specified id, potentially generating/registering it in the process.
func (mb *MetadataBuilder) maybeRegisterDevice(id string, gen func() *Device) int64 {
	// Is this device registered in the larger metadata map?
	if deviceIndex, ok := mb.deviceIndexMap[id]; ok {
		return deviceIndex
	}

	// Generate the device and register it in our Devices list.
	deviceIndex := int64(len(mb.meta.Devices))
	mb.meta.Devices = append(mb.meta.Devices, gen())

	// Add the index to our map for future lookups.
	if mb.deviceIndexMap == nil {
		mb.deviceIndexMap = make(map[string]int64)
	}
	mb.deviceIndexMap[id] = deviceIndex

	return deviceIndex
}

// NumEvents returns the cumulative number of events recorded so far.
func (mb *MetadataBuilder) NumEvents() int64 { return mb.meta.NumEvents }

// NumBytes returns the cumulative number of bytes recorded so far.
func (mb *MetadataBuilder) NumBytes() int64 { return mb.meta.NumBytes }

// Offset returns the current offset in the metadata builder.
func (mb *MetadataBuilder) Offset() time.Duration {
	return mb.cumulativeDuration + mb.currentFileOffset
}

// AddEventFile adds a new event file with the provided parameters to the
// metadata block.
//
// If an event file is currently open, it will be finished and appended to the
// metadata.
func (mb *MetadataBuilder) AddEventFile(name string, comp Compression) {
	mb.finishFileInfo()

	mb.currentFileInfo = &Metadata_EventFile{
		Name:        name,
		Compression: comp,
	}
}

// RecordEvent updates event metadata.
func (mb *MetadataBuilder) RecordEvent(bytes int64, offset time.Duration) {
	if mb.currentFileOffset < offset {
		mb.currentFileOffset = offset
	}

	// Update our current file info stats.
	mb.currentFileInfo.NumEvents++
	mb.currentFileInfo.NumBytes += bytes

	// Update our overall metadata stats.
	mb.meta.NumEvents++
	mb.meta.NumBytes += bytes
}

// Merge adds information from md to mb. Merge will mutate and reference md in
// the process; md should not be used after it has been passed to Merge.
//
// If md is incompatible with mb, Merge will return an error and leave mb in
// an indeterminate state.
func (mb *MetadataBuilder) Merge(md *Metadata) error {
	// If we're in the middle of some other file, we're done now.
	mb.finishFileInfo()

	// Add event files from this Metadata.
	for _, efi := range md.EventFileInfo {
		duration, err := ptypes.Duration(efi.Duration)
		if err != nil {
			return errors.Wrapf(err, "could not parse duration from %s", efi.Duration)
		}
		mb.cumulativeDuration += duration

		mb.meta.NumEvents += efi.NumEvents
		mb.meta.NumBytes += efi.NumBytes

		// Update efi's device map. To do this, we have to resolve its devices
		// within its original Metadata, identify the associated device in our
		// Devices list, and add/map that Device if it doesn't currently exist.
		for i := range efi.DeviceMapping {
			d := efi.DeviceForInternalIndex(int64(i), md)
			if d == nil {
				return errors.Errorf("failed to resolve device #%d", i)
			}

			// Get/register this device index. Make sure it's basically compatible.
			deviceIndex := mb.maybeRegisterDevice(d.Id, func() *Device { return d })
			metaDevice := mb.meta.Devices[deviceIndex]
			if err := assertDevicesCompat(d, metaDevice); err != nil {
				return errors.Wrapf(err, "device %q is not compatible: %s vs. %s", d.Id, d, metaDevice)
			}

			efi.DeviceMapping[i] = deviceIndex
		}
	}
	mb.meta.EventFileInfo = append(mb.meta.EventFileInfo, md.EventFileInfo...)

	return nil
}

// Write writes the constructed metadata to the destination path.
func (mb *MetadataBuilder) Write(path string) error {
	if err := mb.finalize(); err != nil {
		return errors.Wrap(err, "finalizing metadata")
	}

	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if fd != nil {
			// Ignore this error. We explicitly close fd elsewhere in Write and
			// propagate that error; this is just cleanup.
			_ = fd.Close()
		}
	}()

	bio := bufio.NewWriter(fd)

	// Write the protobuf to the file.
	tm := proto.TextMarshaler{}
	if err := tm.Marshal(bio, &mb.meta); err != nil {
		return err
	}

	// Flush our buffer.
	if err := bio.Flush(); err != nil {
		return err
	}

	// Close our file.
	if err := fd.Close(); err != nil {
		return err
	}
	fd = nil // Don't close in defer.

	return nil
}

func (mb *MetadataBuilder) finishFileInfo() {
	if mb.currentFileInfo == nil {
		return
	}

	// Add to event files.
	mb.meta.EventFileInfo = append(mb.meta.EventFileInfo, mb.currentFileInfo)

	// Roll the total duration into "mb".
	mb.cumulativeDuration += mb.currentFileOffset
	mb.currentFileInfo.Duration = ptypes.DurationProto(mb.currentFileOffset)

	// Reset for next file.
	mb.currentFileOffset = 0
	mb.currentFileInfo = nil
	mb.currentDeviceIndexMap = nil
}

// finalize loads any pending metadata into the protobuf.
func (mb *MetadataBuilder) finalize() error {
	// If we have a current file info, finish it.
	mb.finishFileInfo()

	// Add metadata-level protobufs.
	mb.meta.Duration = ptypes.DurationProto(mb.cumulativeDuration)

	// Sort our device list to make it more human-accessible.
	mb.sortDeviceList()

	return nil
}

func (mb *MetadataBuilder) sortDeviceList() {
	// We want to present a sorted device list. However, this will shuffle around
	// our device indexes. We'll track this in the event file's device map.
	//
	// First, map the devices by identity to their current index.
	oldIndexes := make(map[*Device]int64, len(mb.meta.Devices))
	for _, d := range mb.meta.Devices {
		oldIndexes[d] = int64(len(oldIndexes))
	}

	// Sort the device list.
	sort.Sort(sortableDeviceSlice(mb.meta.Devices))

	// Figure out, for any given device index, what the new device index is and
	// map that.
	//
	// While we do this, we track if the indexes actually changed. If they did
	// not, we can avoid updating the event files.
	changed := false
	newIndexes := make(map[int64]int64, len(oldIndexes))
	for i, d := range mb.meta.Devices {
		oldIndex := oldIndexes[d]
		if oldIndex != int64(i) {
			changed = true
		}

		newIndexes[oldIndex] = int64(i)
	}

	// If the indexes changed post-sort, we will update all of the references in
	// all of the event files.
	if changed {
		for _, efi := range mb.meta.EventFileInfo {
			for i, oldDeviceIndex := range efi.DeviceMapping {
				efi.DeviceMapping[i] = newIndexes[oldDeviceIndex]
			}
		}
	}
}

// sortableDeviceSlice is a slice of metadata devices that sorts.
//
// We sort:
// 1) Things with no ordinals (sorted by ID).
// 2) Things with ordinals, sorted first by group, then controller, then ID.
type sortableDeviceSlice []*Device

func (sds sortableDeviceSlice) Len() int      { return len(sds) }
func (sds sortableDeviceSlice) Swap(i, j int) { sds[i], sds[j] = sds[j], sds[i] }
func (sds sortableDeviceSlice) Less(i, j int) bool {
	di, dj := sds[i], sds[j]

	// Devices without ordinals precede Devices with ordinals.
	switch {
	case di.Ordinal == nil:
		if dj.Ordinal != nil {
			// di (no ordinal) < dj(ordinal)
			return true
		}

	case dj.Ordinal == nil:
		// di (ordinal) > dj (no ordinal)
		return false

	default:
		// Both have ordinals, compare their (group, controller).
		switch v := di.Ordinal.Group - dj.Ordinal.Group; {
		case v < 0:
			// di's ordinal group < dj's ordinal group
			return true
		case v > 0:
			// di's ordinal group > dj's ordinal group
			return false
		}

		switch v := di.Ordinal.Controller - dj.Ordinal.Controller; {
		case v < 0:
			// di's ordinal controller < dj's ordinal controller
			return true
		case v > 0:
			// di's ordinal controller > dj's ordinal controller
			return false
		}
	}

	// Either neither have ordinals, or both have equal ordinals. Sort by their
	// IDs.
	return di.Id < dj.Id
}

func assertDevicesCompat(a, b *Device) error {
	switch {
	case a.Id != b.Id:
		return errors.New("unequal IDs")
	case a.PixelsPerStrip != b.PixelsPerStrip:
		return errors.New("pixels per strip do not match")
	case len(a.Strip) != len(b.Strip):
		return errors.New("strip counts do not match")
	default:
		return nil
	}
}
