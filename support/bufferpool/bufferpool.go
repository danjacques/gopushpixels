// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package bufferpool

import (
	"sync"
	"sync/atomic"
)

// Pool maintains a pool of buffers. It offers a new buffer when one is
// unavailable.
type Pool struct {
	// Size is the size of the buffers in this pool.
	Size int

	base sync.Pool
}

// Get returns a buffer, allocating one if one is not available. The returned
// buffer is Reset and returned with a reference count of 1.
//
// The caller should return the buffer to the pool by calling its Release method
// when done with it.
func (bp *Pool) Get() *Buffer {
	b, ok := bp.base.Get().(*Buffer)
	if !ok {
		// Create a blank buffer. When it is released, it will be added back to
		// pool.
		b = &Buffer{
			bytes: make([]byte, bp.Size),
		}
	}

	// Attune the allocated buffer.
	b.pool = bp
	b.size = -1
	b.refcount = 1
	return b
}

func (bp *Pool) releaseNode(b *Buffer) {
	bp.base.Put(b)
}

// Buffer contains a byte buffer that can be released into a Pool for reuse.
//
// Buffer is reference counted, and can be retained and released appropriately.
// Failure to release Buffer will not cause a memory leak, but will prevent the
// reuse of the Buffer.
type Buffer struct {
	refcount int64

	bytes []byte
	size  int

	pool *Pool
	next *Buffer
}

// Bytes returns this buffer's byte slice.
func (b *Buffer) Bytes() []byte {
	if b.size >= 0 {
		return b.bytes[:b.size]
	}
	return b.bytes
}

// Len returns the number of bytes in the buffer.
func (b *Buffer) Len() int { return b.size }

// Truncate artificially caps the number of bytes returned by Bytes.
func (b *Buffer) Truncate(size int) {
	b.size = size
}

// Release returns the buffer to its buffer pool.
//
// Release is safe for concurrent use.
//
// A Buffer must only be released once.
func (b *Buffer) Release() {
	if atomic.AddInt64(&b.refcount, -1) != 0 {
		return
	}

	var pool *Pool
	pool, b.pool = b.pool, nil
	pool.releaseNode(b)
}

// Retain increases the Buffer's reference count. It should be accompanied by
// a Release call to reuse the buffer when it's finished.
func (b *Buffer) Retain() { atomic.AddInt64(&b.refcount, 1) }
