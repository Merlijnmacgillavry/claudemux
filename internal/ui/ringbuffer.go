package ui

import (
	"sync"
)

const defaultRingSize = 512 * 1024 // 512KB

// RingBuffer is a fixed-size byte buffer that drops the oldest data when full.
type RingBuffer struct {
	buf  []byte
	size int
	mu   sync.RWMutex
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buf:  make([]byte, 0, size),
		size: size,
	}
}

func (r *RingBuffer) Write(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, data...)
	if len(r.buf) > r.size {
		r.buf = r.buf[len(r.buf)-r.size:]
	}
}

func (r *RingBuffer) Bytes() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]byte, len(r.buf))
	copy(out, r.buf)
	return out
}

func (r *RingBuffer) String() string {
	return string(r.Bytes())
}

func (r *RingBuffer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = r.buf[:0]
}
