// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package stagingdir

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// D manages a staging directory.
//
// While D is active, it resides in a temporary location. Once finished, D
// can either be committed or destroyed.A On comit, it is atomically moved into
// its destination; on destroy, it is deleted along with all of its contents.
type D struct {
	// tempDir is the temporary directory to use for staging.
	tempDir string

	// path is the path of the staging directory.
	path string
}

// New creates a new staging directory underneath of tempDir.
//
// The directory will be created with the specified prefix.
func New(tempDir, prefix string) (*D, error) {
	// Create a temporary directory.
	stagingPath, err := ioutil.TempDir(tempDir, prefix)
	if err != nil {
		return nil, err
	}

	return &D{
		tempDir: tempDir,
		path:    stagingPath,
	}, nil
}

// Path builds a path relative to the staging directory from the provided
// compoennts.
func (sd *D) Path(first string, components ...string) string {
	if sd.path == "" {
		panic("invalid")
	}

	// Common case: one component undearneath of staging directory.
	if len(components) == 0 {
		return filepath.Join(sd.path, first)
	}

	// Allocate the full set of components for filepath.Join to use.
	comps := make([]string, 0, 2+len(components))
	comps = append(comps, sd.path)
	comps = append(comps, first)
	return filepath.Join(append(comps, components...)...)
}

// Destroy purges the staging directory and its contents.
func (sd *D) Destroy() error {
	if sd.path == "" {
		// There is nothing to destroy.
		return nil
	}

	if err := os.RemoveAll(sd.path); err != nil {
		return err
	}

	sd.path = "" // Destroyed.
	return nil
}

// Commit finalizes the staging directory, atomically moving it to path.
func (sd *D) Commit(dest string) error {
	// If we've already been committed, this is an error.
	if sd.path == "" {
		return errors.New("invalid staging directory")
	}

	// If something already exists at our destination path, delete it.
	if _, st := os.Stat(dest); st == nil {
		// We move the existing file to a directory underneath of our temporary
		// directory. Once created, this will get cleaned up in future iterations
		// regardless, although we'll try and purge it here.
		killDir, err := ioutil.TempDir(sd.tempDir, "overwrite")
		if err != nil {
			return errors.Wrap(err, "create overwrite directory")
		}
		// Purge it in a separate goroutine. If this fails, that's OK - it's under
		// a temporary directory, and will be purged later.
		defer func() {
			go func() {
				_ = os.RemoveAll(killDir)
			}()
		}()

		// Move the existing file into the kill directory. If this fails, we will
		// still try and create the final file, just in case it works.
		killDest := filepath.Join(killDir, filepath.Base(dest))
		_ = os.Rename(dest, killDest)
	}

	// Move the final directory into place (atomic).
	if err := os.Rename(sd.path, dest); err != nil {
		return errors.Wrapf(err, "moving temporary file into place (%q => %q)", sd.path, dest)
	}
	sd.path = "" // Path no longer exists, committed.
	return nil
}
