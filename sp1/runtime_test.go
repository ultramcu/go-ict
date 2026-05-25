// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// errClosed is returned by errorPort after the first successful read.
var errClosed = errors.New("port closed")

// errorPort returns one byte then errors on every subsequent Read call.
// This exercises the Run error-sleep path.
type errorPort struct {
	mu      sync.Mutex
	firstOK bool
	payload byte
	closed  chan struct{}
	once    sync.Once
}

func newErrorPort(b byte) *errorPort {
	return &errorPort{payload: b, closed: make(chan struct{})}
}

func (e *errorPort) Read(p []byte) (int, error) {
	e.mu.Lock()
	if !e.firstOK {
		e.firstOK = true
		p[0] = e.payload
		e.mu.Unlock()
		return 1, nil
	}
	e.mu.Unlock()
	// Block until closed so Run does not spin at 100% CPU.
	select {
	case <-e.closed:
		return 0, io.EOF
	default:
	}
	return 0, errClosed
}

func (e *errorPort) Write(p []byte) (int, error) { return len(p), nil }
func (e *errorPort) Close() error {
	e.once.Do(func() { close(e.closed) })
	return nil
}

// TestRunExitsOnClose verifies that Run unblocks when Close is called, even
// when Read is returning errors (error-sleep path).
func TestRunExitsOnClose(t *testing.T) {
	fp := newErrorPort(0x42)
	d := newFakeDevice(fp)

	runDone := make(chan struct{})
	go func() { d.Run(); close(runDone) }()

	// Let Run hit at least one error-sleep cycle.
	time.Sleep(250 * time.Millisecond)

	d.Close()
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Close (goroutine leak)")
	}
}

// TestRunNilPortReturnsImmediately confirms that Run returns immediately when
// no port is set (the nil-port guard).
func TestRunNilPortReturnsImmediately(t *testing.T) {
	d := &Device{
		queue: newByteQueue(queueCapacity),
		timer: newElapsedTimer(),
		done:  make(chan struct{}),
	}
	done := make(chan struct{})
	go func() { d.Run(); close(done) }()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run with nil port did not return immediately")
	}
}

// TestHardwareWriteErrors exercises the error and short-write paths inside
// hardwareWrite via the drainWriteQueue goroutine.
type badWritePort struct {
	mu       sync.Mutex
	written  []byte
	failNext bool
	shortN   int // if >0, report this many bytes written (short write)
	closed   chan struct{}
	once     sync.Once
}

func newBadWritePort() *badWritePort {
	return &badWritePort{closed: make(chan struct{})}
}

func (b *badWritePort) Read(p []byte) (int, error) {
	<-b.closed
	return 0, io.EOF
}

func (b *badWritePort) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failNext {
		b.failNext = false
		return 0, errors.New("injected write error")
	}
	if b.shortN > 0 && b.shortN < len(p) {
		n := b.shortN
		b.shortN = 0
		b.written = append(b.written, p[:n]...)
		return n, nil // short write
	}
	b.written = append(b.written, p...)
	return len(p), nil
}

func (b *badWritePort) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

// TestHardwareWriteErrorLogged verifies that a write error is logged (the
// error branch in hardwareWrite).
func TestHardwareWriteErrorLogged(t *testing.T) {
	bp := newBadWritePort()
	bp.failNext = true

	d := newFakeDevice(bp)
	cl := &collectingLogf{}
	d.SetLogFunc(cl.fn())

	// Inject one byte so drainWriteQueue calls hardwareWrite once.
	go d.drainWriteQueue()
	d.Write([]byte{0x41}) // 'A'

	// Wait up to 500 ms for the log line.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap := cl.snapshot()
		for _, l := range snap {
			if containsAll(l, "serial write failed") {
				d.Close()
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	d.Close()

	// If we get here without finding the log, fail.
	for _, l := range cl.snapshot() {
		if containsAll(l, "serial write failed") {
			return
		}
	}
	t.Errorf("expected 'serial write failed' log line, got: %v", cl.snapshot())
}

// TestHardwareWriteShortWriteLogged verifies the short-write log branch.
func TestHardwareWriteShortWriteLogged(t *testing.T) {
	// We need to send >1 byte in a single hardwareWrite call so we can trigger
	// a short write. hardwareWrite is called one-byte-at-a-time by drainWriteQueue,
	// so it never short-writes via normal flow. We call it directly instead.
	bp := newBadWritePort()
	bp.shortN = 1 // report only 1 byte written out of 2

	d := newFakeDevice(bp)
	cl := &collectingLogf{}
	d.SetLogFunc(cl.fn())

	// Call hardwareWrite directly with a 2-byte slice.
	d.hardwareWrite([]byte{0x41, 0x42})

	// Verify log.
	found := false
	for _, l := range cl.snapshot() {
		if containsAll(l, "short write") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'short write' log line, got: %v", cl.snapshot())
	}
}

// TestDrainWriteQueue_CRGap verifies that after a CR the drainer enforces
// a commitGap before the next byte. We record Write timestamps and assert
// the gap is ≥ commitGap (3 s). This test is inherently slow and should be
// skipped in short mode.
func TestDrainWriteQueue_CRGap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 3-second commit-gap test in -short mode")
	}

	type writeEvent struct {
		data []byte
		at   time.Time
	}
	var evMu sync.Mutex
	var events []writeEvent

	rp := &recordingPort{
		closed: make(chan struct{}),
		onWrite: func(p []byte, at time.Time) {
			evMu.Lock()
			events = append(events, writeEvent{append([]byte(nil), p...), at})
			evMu.Unlock()
		},
	}

	d := newFakeDevice(rp)
	d.timer = newElapsedTimer()
	go d.drainWriteQueue()

	d.Write([]byte{CR, 'X'})

	// Wait up to 4 s for both bytes to be written.
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		evMu.Lock()
		n := len(events)
		evMu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	d.Close()

	evMu.Lock()
	snap := append([]writeEvent(nil), events...)
	evMu.Unlock()

	if len(snap) < 2 {
		t.Fatalf("expected 2 write events, got %d", len(snap))
	}
	if snap[0].data[0] != CR {
		t.Errorf("first write = 0x%02X, want CR (0x0D)", snap[0].data[0])
	}
	if snap[1].data[0] != 'X' {
		t.Errorf("second write = 0x%02X, want 'X'", snap[1].data[0])
	}
	gap := snap[1].at.Sub(snap[0].at)
	if gap < commitGap {
		t.Errorf("gap after CR = %v, want >= %v", gap, commitGap)
	}
}

// TestHardwareWriteNilPort verifies that hardwareWrite is a safe no-op when
// the Device has no port (d.port == nil).
func TestHardwareWriteNilPort(t *testing.T) {
	d := &Device{
		queue: newByteQueue(queueCapacity),
		timer: newElapsedTimer(),
		done:  make(chan struct{}),
		// port intentionally nil
	}
	// Must not panic.
	d.hardwareWrite([]byte{0x41})
}

// TestDrainWriteQueue_CROrdering verifies that bytes arrive at the port in
// FIFO order (CR first, then subsequent bytes). This fast variant does not
// assert the timing gap.
func TestDrainWriteQueue_CROrdering(t *testing.T) {
	var mu sync.Mutex
	var received []byte

	rp := &recordingPort{
		closed: make(chan struct{}),
		onWrite: func(p []byte, _ time.Time) {
			mu.Lock()
			received = append(received, p...)
			mu.Unlock()
		},
	}

	d := newFakeDevice(rp)
	go d.drainWriteQueue()

	// Write non-CR bytes only so no commitGap delay is triggered.
	d.Write([]byte{'A', 'B', 'C'})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	d.Close()

	mu.Lock()
	got := append([]byte(nil), received...)
	mu.Unlock()

	if string(got) != "ABC" {
		t.Errorf("port received %q, want %q", got, "ABC")
	}
}

// recordingPort records Write calls with timestamps.
type recordingPort struct {
	onWrite func(p []byte, at time.Time)
	closed  chan struct{}
	once    sync.Once
}

func (r *recordingPort) Read(p []byte) (int, error) {
	<-r.closed
	return 0, io.EOF
}

func (r *recordingPort) Write(p []byte) (int, error) {
	if r.onWrite != nil {
		r.onWrite(p, time.Now())
	}
	return len(p), nil
}

func (r *recordingPort) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

// containsAll reports whether s contains all the given substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
