// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// collectingLogf captures every log call into a slice (thread-safe).
type collectingLogf struct {
	mu   sync.Mutex
	msgs []string
}

func (c *collectingLogf) fn() Logf {
	return func(format string, args ...interface{}) string {
		var msg string
		if len(args) > 0 {
			// The internal helpers always call logf("[TAG] : %s", body)
			if s, ok := args[0].(string); ok {
				msg = format + s
			}
		} else {
			msg = format
		}
		c.mu.Lock()
		c.msgs = append(c.msgs, msg)
		c.mu.Unlock()
		return msg
	}
}

func (c *collectingLogf) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.msgs...)
}

// TestSetLogFunc verifies installing and removing a log function controls
// whether log output is produced.
func TestSetLogFunc(t *testing.T) {
	d := newQueueDevice()
	cl := &collectingLogf{}

	// Before installation: no log output.
	d.log("no-op %d\r\n", 1)
	if snap := cl.snapshot(); len(snap) != 0 {
		t.Fatalf("expected no log before SetLogFunc, got %v", snap)
	}

	// Install the logger.
	d.SetLogFunc(cl.fn())
	d.log("generic\r\n")
	d.logSP1("sp1 msg\r\n")
	d.logCPU("cpu msg\r\n")

	snap := cl.snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 log lines after SetLogFunc, got %d: %v", len(snap), snap)
	}
	for i, tag := range []string{"[   ]", "[SP1]", "[CPU]"} {
		if !strings.Contains(snap[i], tag) {
			t.Errorf("line %d: want tag %q, got %q", i, tag, snap[i])
		}
	}

	// Remove the logger: no more output.
	d.SetLogFunc(nil)
	d.log("should be silent\r\n")
	if snap2 := cl.snapshot(); len(snap2) != 3 {
		t.Errorf("expected no new log after SetLogFunc(nil), got %v", snap2[3:])
	}
}

// TestLogTagRouting verifies that log/logSP1/logCPU each prepend the correct tag.
func TestLogTagRouting(t *testing.T) {
	cases := []struct {
		name string
		call func(*Device)
		tag  string
	}{
		{"log", func(d *Device) { d.log("hello %s\r\n", "world") }, "[   ]"},
		{"logSP1", func(d *Device) { d.logSP1("rx byte\r\n") }, "[SP1]"},
		{"logCPU", func(d *Device) { d.logCPU("cmd sent\r\n") }, "[CPU]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newQueueDevice()
			cl := &collectingLogf{}
			d.SetLogFunc(cl.fn())
			tc.call(d)

			snap := cl.snapshot()
			if len(snap) != 1 {
				t.Fatalf("expected 1 log line, got %d", len(snap))
			}
			if !strings.HasPrefix(snap[0], tc.tag) {
				t.Errorf("%s: got %q, want prefix %q", tc.name, snap[0], tc.tag)
			}
		})
	}
}

// oneBytePort returns one payload byte on the first Read then blocks until Close.
type oneBytePort struct {
	payload byte
	sent    bool
	mu      sync.Mutex
	closed  chan struct{}
	once    sync.Once
}

func newOneBytePort(b byte) *oneBytePort {
	return &oneBytePort{payload: b, closed: make(chan struct{})}
}

func (o *oneBytePort) Read(p []byte) (int, error) {
	o.mu.Lock()
	if !o.sent {
		o.sent = true
		p[0] = o.payload
		o.mu.Unlock()
		return 1, nil
	}
	o.mu.Unlock()
	<-o.closed
	return 0, io.EOF
}

func (o *oneBytePort) Write(p []byte) (int, error) { return len(p), nil }

func (o *oneBytePort) Close() error {
	o.once.Do(func() { close(o.closed) })
	return nil
}

// TestRunLogsRXBytes verifies that Run emits a log line containing the
// received byte when logf is installed.
func TestRunLogsRXBytes(t *testing.T) {
	fp := newOneBytePort(0xAB)
	d := newFakeDevice(fp)

	cl := &collectingLogf{}
	d.SetLogFunc(cl.fn())

	runDone := make(chan struct{})
	go func() { d.Run(); close(runDone) }()

	// Wait up to 500 ms for at least one log line to appear.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if snap := cl.snapshot(); len(snap) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	d.Close()
	<-runDone

	found := false
	for _, l := range cl.snapshot() {
		if strings.Contains(l, "0xAB") || strings.Contains(l, "RX") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected RX log for byte 0xAB; got: %v", cl.snapshot())
	}
}
