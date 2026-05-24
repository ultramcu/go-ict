// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"io"
	"sync"
	"testing"
	"time"
)

// fakePort is an in-memory io.ReadWriteCloser. Read blocks until Close,
// mimicking an idle serial device.
type fakePort struct {
	mu      sync.Mutex
	written []byte
	closed  chan struct{}
	once    sync.Once
}

func newFakePort() *fakePort { return &fakePort{closed: make(chan struct{})} }

func (f *fakePort) Read(p []byte) (int, error) {
	<-f.closed
	return 0, io.EOF
}

func (f *fakePort) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.closed:
		return 0, io.ErrClosedPipe
	default:
	}
	f.written = append(f.written, p...)
	return len(p), nil
}

func (f *fakePort) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func (f *fakePort) bytes() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.written...)
}

func newFakeDevice(p io.ReadWriteCloser) *Device {
	return &Device{
		port:  p,
		queue: newByteQueue(queueCapacity),
		timer: newElapsedTimer(),
		done:  make(chan struct{}),
	}
}

// TestWriteAfterCloseNoPanic guards the historical send-on-closed-channel
// crash: Write after Close must be a safe no-op, not a panic.
func TestWriteAfterCloseNoPanic(t *testing.T) {
	d := newFakeDevice(newFakePort())
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if n := d.Write([]byte{1, 2, 3}); n != 0 {
		t.Errorf("Write after Close enqueued %d, want 0", n)
	}
	if n := d.WriteString("x"); n != 0 {
		t.Errorf("WriteString after Close enqueued %d, want 0", n)
	}
}

// TestConcurrentWriteClose runs Write and Close concurrently; with -race
// this surfaces the data/shutdown race the queue rework fixed.
func TestConcurrentWriteClose(t *testing.T) {
	d := newFakeDevice(newFakePort())
	go d.drainWriteQueue()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); d.Write([]byte{0x41}) }()
	}
	d.Close()
	wg.Wait() // must complete without panicking
}

// TestCloseStopsRun verifies Close interrupts a Run blocked in a read.
func TestCloseStopsRun(t *testing.T) {
	d := newFakeDevice(newFakePort())
	done := make(chan struct{})
	go func() { d.Run(); close(done) }()

	time.Sleep(20 * time.Millisecond)
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Close (goroutine leak)")
	}
}

// TestDrainWritesToPort checks queued bytes reach the port in order.
func TestDrainWritesToPort(t *testing.T) {
	fp := newFakePort()
	d := newFakeDevice(fp)
	go d.drainWriteQueue()

	d.Write([]byte("AB")) // no CR, so no commit-gap delay

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && len(fp.bytes()) < 2 {
		time.Sleep(5 * time.Millisecond)
	}
	d.Close()

	if got := string(fp.bytes()); got != "AB" {
		t.Errorf("port received %q, want %q", got, "AB")
	}
}
