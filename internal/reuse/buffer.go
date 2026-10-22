package reuse

import (
	"bytes"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// GetBuffer retrieves a clean bytes.Buffer from the global sync.Pool.
func GetBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutBuffer resets and returns a bytes.Buffer to the global sync.Pool for zero-alloc re-use.
func PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	// Avoid hoarding oversized buffers (>64KB) in the pool
	if buf.Cap() > 64*1024 {
		return
	}
	buf.Reset()
	bufferPool.Put(buf)
}
