// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import "sync"

// byteQueue is a small bounded FIFO of bytes built on a buffered
// channel. It is unexported -- callers go through Device.Write /
// WriteString -- and exists so the public Write* methods can be
// non-blocking up to the queue's capacity while a single drainer
// goroutine handles the protocol-level pacing.
//
// Shutdown is signalled with a separate done channel rather than by
// closing the data channel, so a Write racing Close never panics with
// "send on closed channel".
type byteQueue struct {
	ch        chan byte
	done      chan struct{}
	closeOnce sync.Once
}

func newByteQueue(capacity int) *byteQueue {
	return &byteQueue{
		ch:   make(chan byte, capacity),
		done: make(chan struct{}),
	}
}

// Enqueue pushes one byte, blocking if the queue is full (back-pressure).
// It returns false if the queue has been closed; it never panics, so it
// is safe to call concurrently with (or after) Close.
func (q *byteQueue) Enqueue(b byte) bool {
	// Fast path: bail out immediately if already closed.
	select {
	case <-q.done:
		return false
	default:
	}
	select {
	case q.ch <- b:
		return true
	case <-q.done:
		return false
	}
}

// Dequeue pops one byte. After Close it keeps returning any bytes still
// buffered (so an in-flight job drains) and only reports ok=false once
// the buffer is empty.
func (q *byteQueue) Dequeue() (b byte, ok bool) {
	// Prefer a buffered byte, even if the queue is already closed.
	select {
	case b = <-q.ch:
		return b, true
	default:
	}
	select {
	case b = <-q.ch:
		return b, true
	case <-q.done:
		return 0, false
	}
}

// Close stops the queue: in-flight and future Enqueue calls return
// false and Dequeue returns ok=false. Idempotent and never panics.
func (q *byteQueue) Close() {
	q.closeOnce.Do(func() { close(q.done) })
}
