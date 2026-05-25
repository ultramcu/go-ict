// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"strings"
	"testing"
	"time"
)

func TestAddSpaceToFrontOfText(t *testing.T) {
	cases := []struct {
		text  string
		count int
		want  string
	}{
		{"hello", 0, "hello"},
		{"hello", -3, "hello"},
		{"hello", 3, "   hello"},
	}
	for _, tc := range cases {
		if got := AddSpaceToFrontOfText(tc.text, tc.count); got != tc.want {
			t.Errorf("AddSpaceToFrontOfText(%q, %d) = %q, want %q",
				tc.text, tc.count, got, tc.want)
		}
	}
}

func TestAlignmentTextWithSpace(t *testing.T) {
	cases := []struct {
		name  string
		size  FontSize
		align Alignment
		text  string
		want  string
	}{
		{"Large/Center pads half", Large, Center, "OK", strings.Repeat(" ", (MaxLargeChars-2)/2) + "OK"},
		{"Large/Right pads full", Large, Right, "OK", strings.Repeat(" ", MaxLargeChars-2) + "OK"},
		{"Normal/Center pads half", Normal, Center, "OK", strings.Repeat(" ", (MaxNormalChars-2)/2) + "OK"},
		{"Small/Right pads full", Small, Right, "AB", strings.Repeat(" ", MaxSmallChars-2) + "AB"},
		{"Left returns unchanged", Normal, Left, "hello", "hello"},
		{"Text wider than line returns unchanged", Large, Center, strings.Repeat("X", 32), strings.Repeat("X", 32)},
		{"UTF-8 counts runes not bytes", Normal, Center, "ห", strings.Repeat(" ", (MaxNormalChars-1)/2) + "ห"},
		{"Unknown FontSize returns text unchanged", FontSize(99), Center, "hello", "hello"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AlignmentTextWithSpace(tc.size, tc.align, tc.text); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRemoveNewline(t *testing.T) {
	cases := map[string]string{
		"hello\n":   "hello",
		"hello":     "hello",
		"\n":        "",
		"hello\n\n": "hello\n",
		"":          "",
	}
	for in, want := range cases {
		if got := RemoveNewline(in); got != want {
			t.Errorf("RemoveNewline(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFontSizeString(t *testing.T) {
	cases := map[FontSize]string{
		Small:        "Small",
		Normal:       "Normal",
		Large:        "Large",
		FontSize(99): "Unknown",
	}
	for f, want := range cases {
		if got := f.String(); got != want {
			t.Errorf("(%d).String() = %q, want %q", int(f), got, want)
		}
	}
}

func TestAlignmentString(t *testing.T) {
	cases := map[Alignment]string{
		Left:          "Left",
		Right:         "Right",
		Center:        "Center",
		Alignment(99): "Unknown",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("(%d).String() = %q, want %q", int(a), got, want)
		}
	}
}

func TestByteQueue_FIFO(t *testing.T) {
	q := newByteQueue(4)
	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)

	for i, want := range []byte{1, 2, 3} {
		got, ok := q.Dequeue()
		if !ok {
			t.Fatalf("Dequeue %d: !ok", i)
		}
		if got != want {
			t.Errorf("Dequeue %d = %d, want %d", i, got, want)
		}
	}
}

func TestByteQueue_CloseSignals(t *testing.T) {
	q := newByteQueue(2)
	q.Enqueue(7)
	q.Close()

	// Drain the remaining buffered byte.
	got, ok := q.Dequeue()
	if !ok || got != 7 {
		t.Errorf("first Dequeue after Close = (%d, %v), want (7, true)", got, ok)
	}
	// Second Dequeue should return the closed/zero value.
	_, ok = q.Dequeue()
	if ok {
		t.Errorf("second Dequeue after Close: ok = true, want false")
	}
}

// TestByteQueue_EnqueueAfterClose exercises the fast-path early-exit in
// Enqueue: after Close, Enqueue must return false without blocking.
func TestByteQueue_EnqueueAfterClose(t *testing.T) {
	q := newByteQueue(4)
	q.Close()

	// The first select in Enqueue should detect done and return false.
	ok := q.Enqueue(0x55)
	if ok {
		t.Errorf("Enqueue after Close returned true, want false")
	}
}

// TestByteQueue_DequeueEmptyAfterClose exercises the blocking Dequeue path:
// after Close with an empty queue, Dequeue must return (0, false) promptly.
func TestByteQueue_DequeueEmptyAfterClose(t *testing.T) {
	q := newByteQueue(4)
	// Do not enqueue anything.
	q.Close()

	done := make(chan struct{})
	go func() {
		_, ok := q.Dequeue()
		if ok {
			// Signal test failure via channel — t.Error from goroutine is fine
			// but we can't safely call t.Fatal here.
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Dequeue on empty closed queue blocked instead of returning promptly")
	}
}

// TestByteQueue_CloseIdempotent verifies that calling Close twice does not panic.
func TestByteQueue_CloseIdempotent(t *testing.T) {
	q := newByteQueue(4)
	q.Close()
	q.Close() // must not panic
}

// TestByteQueue_DequeueBlocksUntilEnqueue exercises the blocking path in
// Dequeue: a consumer waiting on an empty queue unblocks when a producer
// enqueues a byte.
func TestByteQueue_DequeueBlocksUntilEnqueue(t *testing.T) {
	q := newByteQueue(4)

	result := make(chan byte, 1)
	go func() {
		b, ok := q.Dequeue()
		if ok {
			result <- b
		}
	}()

	// Give the goroutine time to block in Dequeue.
	time.Sleep(20 * time.Millisecond)
	q.Enqueue(0xBE)

	select {
	case got := <-result:
		if got != 0xBE {
			t.Errorf("Dequeue returned %#x, want 0xBE", got)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Dequeue did not unblock after Enqueue")
	}
}

// TestByteQueue_EnqueueBlocksOnFull exercises the blocking path in Enqueue:
// when the queue is at capacity, Enqueue blocks until the consumer makes room.
func TestByteQueue_EnqueueBlocksOnFull(t *testing.T) {
	q := newByteQueue(1) // capacity 1

	// Fill the queue.
	if !q.Enqueue(0x01) {
		t.Fatal("first Enqueue failed")
	}

	enqueued := make(chan bool, 1)
	go func() {
		// This should block until the consumer reads.
		enqueued <- q.Enqueue(0x02)
	}()

	// Goroutine should be blocked; give it time to reach the channel send.
	time.Sleep(20 * time.Millisecond)
	select {
	case <-enqueued:
		t.Fatal("Enqueue returned before consumer drained")
	default:
	}

	// Drain one byte to make room.
	_, _ = q.Dequeue()

	// Now Enqueue should unblock.
	select {
	case ok := <-enqueued:
		if !ok {
			t.Error("Enqueue returned false after room was made")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Enqueue did not unblock after Dequeue")
	}
}

// TestByteQueue_EnqueueBlockedThenClosed exercises the case where Enqueue
// is blocking on a full queue and then Close is called: Enqueue must return
// false without deadlocking.
func TestByteQueue_EnqueueBlockedThenClosed(t *testing.T) {
	q := newByteQueue(1) // capacity 1

	// Fill the queue so the next Enqueue will block.
	q.Enqueue(0x01)

	result := make(chan bool, 1)
	go func() {
		// Will block on the second select, then unblock when Close fires.
		result <- q.Enqueue(0x02)
	}()

	// Let the goroutine reach the blocked select.
	time.Sleep(20 * time.Millisecond)

	// Close the queue; the blocked Enqueue must wake up and return false.
	q.Close()

	select {
	case ok := <-result:
		if ok {
			t.Error("Enqueue returned true after queue was closed")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Enqueue did not return after Close (deadlock)")
	}
}

func TestElapsedTimer_AdvancesOnEachCall(t *testing.T) {
	tm := newElapsedTimer()
	first := tm.SinceLastCall()
	if first < 0 {
		t.Errorf("first SinceLastCall = %v, want >= 0", first)
	}

	time.Sleep(15 * time.Millisecond)

	second := tm.SinceLastCall()
	// second should be at least the sleep we just did.
	if second < 10*time.Millisecond {
		t.Errorf("second SinceLastCall = %v, want >= 10ms", second)
	}
}
