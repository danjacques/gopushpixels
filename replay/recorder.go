// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package replay

import (
	"sync"
	"time"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/replay/streamfile"

	"github.com/pkg/errors"
)

// RecorderStatus is a snapshot of the current recorder status.
type RecorderStatus struct {
	Name     string
	Error    error
	Events   int64
	Bytes    int64
	Duration time.Duration
}

// A Recorder handles the recoridng and playback of packets.
type Recorder struct {
	mu sync.Mutex
	// sw is the currently-active stream writer.
	sw *streamfile.EventStreamWriter
	// stopped is true if we've been stopped.
	stopped bool
	// err is an error that occurred while receiving a packet.
	recvErr error
}

// Start starts recording a stream.
//
// The recording will continue until the Stop method is called.
//
// Start will take ownership of sw and close it on completion (Stop).
func (r *Recorder) Start(sw *streamfile.EventStreamWriter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sw != nil {
		panic("already started")
	}

	// Register this recorder with our Proxy.
	r.sw = sw
	recorderRecordingGauge.Inc()
}

// Stop stops the Recorder, finalizing its output file and releasing its
// resources.
func (r *Recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sw == nil {
		return nil
	}

	// Finalize our recorded file.
	err := r.sw.Close()
	r.sw = nil

	// Propagate our receive error, if Close didn't return an error.
	if err == nil {
		err = r.recvErr
	}
	r.recvErr = nil

	recorderRecordingGauge.Dec()
	return err
}

// Status returns a snapshot of the current Recorder status.
//
// If the Recorder is not currently recording, Status will return nil.
func (r *Recorder) Status() *RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sw == nil {
		return nil
	}

	return &RecorderStatus{
		Name:     r.sw.Path(),
		Error:    r.recvErr,
		Events:   r.sw.NumEvents(),
		Bytes:    r.sw.NumBytes(),
		Duration: r.sw.Duration(),
	}
}

// RecordPacket adds pkt from device d to the recording.
func (r *Recorder) RecordPacket(d device.D, pkt *protocol.Packet) error {
	recorderEvents.Inc()

	r.mu.Lock()
	defer r.mu.Unlock()

	// If we've been stopped, but not yet unregistered, then do nothing.
	if r.sw == nil {
		return nil
	}

	// We're already in an error state.
	switch err := r.sw.WritePacket(d, pkt); errors.Cause(err) {
	case nil:
		// Write succeeded!
		return nil

	case streamfile.ErrEncodingNotSupported:
		// This packet contained an unsupported data type. Ignore it.
		recorderErrors.WithLabelValues("encoding").Inc()
		return err

	default:
		// Record the error. We're done; let's not waste time on more packets.
		recorderErrors.WithLabelValues("unknown").Inc()
		r.recvErr = err
		return err
	}
}
