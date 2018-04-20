// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package streamfile

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"os"

	"github.com/danjacques/gopushpixels/support/dataio"

	"github.com/golang/snappy"
	"github.com/pkg/errors"
)

const (
	// Large buffer size (4MB), good for reading the file.
	rawStreamLargeBufferSize = 1024 * 1024 * 4
)

var nopReader = bytes.NewReader(nil)

type rawStreamReader struct {
	// Currently connected to the source reader.
	dataio.Reader

	br      *bufio.Reader
	snappyR *snappy.Reader
	gzipR   *gzip.Reader
}

func (r *rawStreamReader) reset(base io.Reader, comp Compression) error {
	if r.br == nil {
		r.br = bufio.NewReaderSize(base, rawStreamLargeBufferSize)
	} else {
		r.br.Reset(base)
	}

	switch comp {
	case Compression_SNAPPY:
		if r.snappyR == nil {
			r.snappyR = snappy.NewReader(r.br)
		} else {
			r.snappyR.Reset(r.br)
		}
		r.Reader = dataio.MakeReader(r.snappyR)

	case Compression_GZIP:
		if r.gzipR == nil {
			gz, err := gzip.NewReader(r.br)
			if err != nil {
				return errors.Wrap(err, "creating gzip reader")
			}
			r.gzipR = gz
		} else {
			if err := r.gzipR.Reset(r.br); err != nil {
				return errors.Wrap(err, "resetting gzip reader")
			}
		}
		r.Reader = dataio.MakeReader(r.gzipR)

	case Compression_NONE:
		r.Reader = r.br

	default:
		return errors.Errorf("unknown compression: %s", comp)
	}
	return nil
}

type rawStreamWriter struct {
	dataio.Writer

	closer  io.Closer
	bw      *bufio.Writer
	snappyW *snappy.Writer
	gzipW   *gzip.Writer
}

func newRawStreamWriter(base io.WriteCloser) *rawStreamWriter {
	w := rawStreamWriter{
		bw:     bufio.NewWriterSize(base, rawStreamLargeBufferSize),
		closer: base,
	}
	w.Writer = w.bw
	return &w
}

func (w *rawStreamWriter) beginCompression(comp Compression, level int) error {
	switch comp {
	case Compression_SNAPPY:
		w.snappyW = snappy.NewBufferedWriter(w.bw)
		w.Writer = dataio.MakeWriter(w.snappyW)

	case Compression_GZIP:
		if level < 0 {
			level = gzip.DefaultCompression
		}

		gw, err := gzip.NewWriterLevel(w.bw, level)
		if err != nil {
			return errors.Wrap(err, "creatign gzip writer")
		}
		w.gzipW = gw
		w.Writer = dataio.MakeWriter(w.gzipW)

	case Compression_NONE:
		w.Writer = w.bw

	default:
		return errors.Errorf("unknown compression: %s", comp)
	}
	return nil
}

func (w *rawStreamWriter) Close() (err error) {
	// Always close our underlying base, if we have one.
	if w.closer != nil {
		defer func() {
			closeErr := w.closer.Close()
			if err == nil {
				err = closeErr
			}
		}()
	}

	if w.snappyW != nil {
		if err = w.snappyW.Close(); err != nil {
			return
		}
	}
	if w.gzipW != nil {
		if err = w.gzipW.Close(); err != nil {
			return
		}
	}

	if err = w.bw.Flush(); err != nil {
		return
	}

	// Return (our "closer" will be closed in defer).
	return
}

// hardLinkOrCopy attempts to make dest the same file as src.
//
// Ideally, it will use a hard link. If that fails, it will fall back to
// byte-by-byte copying.
func hardLinkOrCopy(src, dest string) error {
	// First, try and hard-link.
	if err := os.Link(src, dest); err == nil {
		return nil
	}

	// Fall back to copying.
	return copyFileByteByByte(src, dest)
}

func copyFileByteByByte(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		if out != nil {
			_ = out.Close()
		}
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	out = nil // Don't double-close in defer.
	return nil
}
