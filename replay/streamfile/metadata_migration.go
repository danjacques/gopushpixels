// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"github.com/pkg/errors"
)

// migrateMetadata migrates md from its version to the current major version.
//
// This reduces the in-app logic paths that need to operate.
//
// Migration moves the Metadata forwards one version at a time until it reaches
// the latest version. If there an error, it will be returned.
func migrateMetadata(md *Metadata) error {
	for md.Minor != metadataMinorVersion {
		curVersion := md.Minor

		switch curVersion {
		case 0:
			// Migrate to v1.
			if err := migrateMetadata0_1(md); err != nil {
				return err
			}
		}

		// Enforce that each migration step must advance the version.
		if md.Minor <= curVersion {
			return errors.New("migration did not advance version")
		}
	}
	return nil
}

func migrateMetadata0_1(md *Metadata) error {
	// Merge old "event_file" into new "event_file_info".
	// TODO: Remove this once all files use "event_file_info".
	for _, ef := range md.EventFile {
		md.EventFileInfo = append(md.EventFileInfo, &Metadata_EventFile{
			Name:        ef,
			Compression: md.Compression,
			Duration:    md.Duration,
			NumBytes:    md.NumBytes,
			NumEvents:   md.NumEvents,
		})
	}
	md.EventFile = nil

	// Patch the Devices list in the event file.
	// TODO: Remove this once all files use event file level devices.
	var identityMap []int64
	for _, efi := range md.EventFileInfo {
		if len(efi.DeviceMapping) != 0 {
			continue
		}

		// If our identity map hasn't been populated, populate it now.
		if identityMap == nil {
			identityMap = make([]int64, len(md.Devices))
			for i := range md.Devices {
				identityMap[i] = int64(i)
			}
		}

		efi.DeviceMapping = identityMap
	}

	md.Minor = 1
	return nil
}
