package ringbuffer

import (
	"container/ring"
	"fmt"
)

// RingBuffer holds the ring buffer and the synchronization primitives
type RingBuffer struct {
	read     *ring.Ring
	write    *ring.Ring
	capacity int
	size     int
	overflow int
}

// New creates new ring buffer
func New(capacity int) *RingBuffer {
	r := ring.New(capacity)
	return &RingBuffer{
		read:     r,
		write:    r,
		capacity: capacity,
	}
}

// Push inserts new value to ring buffer.
// Overflow signals if ring buffer was full when Push was called.
func (r *RingBuffer) Push(value interface{}) {
	r.write.Value = value

	// Advance write pointer
	r.write = r.write.Next()

	// If ring buffer became full, advance read pointer
	if r.full() {
		r.read = r.read.Next()
		r.overflow++
	} else {
		r.size++
	}
}

// Pop removes value from ring buffer.
func (r *RingBuffer) Pop() (interface{}, error) {
	// If ring buffer is empty, wait for producer to fill values into it
	if r.Empty() {
		return nil, fmt.Errorf("RingBuffer is empty")
	}
	// Advance read pointer
	v := r.read.Value
	r.read = r.read.Next()
	r.size--
	r.overflow = 0

	return v, nil
}

func (r *RingBuffer) Empty() bool {
	return r.read == r.write
}

func (r *RingBuffer) full() bool {
	return r.size == r.capacity
}
