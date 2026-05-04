// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

// byteQueue is a small bounded FIFO of bytes built on a buffered
// channel. It is unexported -- callers go through Device.Write /
// WriteString -- and exists so the public Write* methods can be
// non-blocking up to the queue's capacity while a single drainer
// goroutine handles the protocol-level pacing.
//
// Enqueue blocks if the queue is full (back-pressure). The drainer
// blocks on dequeue when the queue is empty.
type byteQueue struct {
	ch chan byte
}

func newByteQueue(capacity int) *byteQueue {
	return &byteQueue{ch: make(chan byte, capacity)}
}

// Enqueue pushes one byte. Blocks if the queue is full.
func (q *byteQueue) Enqueue(b byte) { q.ch <- b }

// Dequeue pops one byte. Blocks if the queue is empty.
// Returns ok=false if the queue has been closed and drained.
func (q *byteQueue) Dequeue() (b byte, ok bool) {
	b, ok = <-q.ch
	return
}

// Close releases waiters of Dequeue. Subsequent Enqueue panics.
func (q *byteQueue) Close() { close(q.ch) }
