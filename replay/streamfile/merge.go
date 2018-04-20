// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"fmt"
	"path/filepath"

	"github.com/danjacques/gopushpixels/support/stagingdir"

	"github.com/pkg/errors"
)

// Merge merges the stream files at the specified paths together to form a
// stream file at dest.
//
// Ideally, the merge will be relatively instant, using hardlinks to clone the
// source data.
func (cfg *EventStreamConfig) Merge(dest, displayName string, paths ...string) error {
	mb, err := cfg.NewMetadataBuilder(displayName)
	if err != nil {
		return errors.Wrap(err, "creating metadata builder")
	}

	// Load all metadata from paths.
	//
	// Use a cache so we don't waste time double-loading the same path, in case
	// it is repeated multiple times.
	mdCache := make(map[string]*Metadata, len(paths))
	metadatas := make([]*Metadata, len(paths))
	for i, p := range paths {
		md := mdCache[p]
		if md == nil {
			md = &Metadata{}
			if err := LoadMetadata(p, md); err != nil {
				return errors.Wrapf(err, "loading metadata for %q", p)
			}
			mdCache[p] = md
		}

		metadatas[i] = md
	}

	// Create a staging directory for the merge.
	stagingDir, err := stagingdir.New(cfg.TempDir, filepath.Base(dest))
	if err != nil {
		return errors.Wrap(err, "could not create staging directory")
	}
	defer func() {
		_ = stagingDir.Destroy()
	}()

	// Link/copy all of the source event files into the staging directory.
	copied := make(map[*Metadata]struct{}, len(mdCache))
	for i, md := range metadatas {
		// If we've already handled files for this Metadata, skip it.
		if _, ok := copied[md]; ok {
			continue
		}
		copied[md] = struct{}{}

		// Iterate through each event file in this Metadata.
		for j, efi := range md.EventFileInfo {
			srcPath := filepath.Join(paths[i], efi.Name)

			// Update efi's Name with the destination file.
			efi.Name = fmt.Sprintf("merged.%d.%d"+eventFileExt, i, j)
			dstPath := stagingDir.Path(efi.Name)

			if err := hardLinkOrCopy(srcPath, dstPath); err != nil {
				return errors.Wrapf(err, "could not link %q => %q", srcPath, dstPath)
			}
		}
	}

	// Build our composite metadata.
	for i, md := range metadatas {
		if err := mb.Merge(md); err != nil {
			return errors.Wrapf(err, "merging metadata for %q", paths[i])
		}
	}

	// Write the final Metadata back out to the staging directory.
	if err := mb.Write(stagingDir.Path(metadataFileName)); err != nil {
		return errors.Wrap(err, "writing metadata")
	}

	// Commit the staging directory.
	if err := stagingDir.Commit(dest); err != nil {
		return errors.Wrap(err, "committing staging directory")
	}
	return nil
}
