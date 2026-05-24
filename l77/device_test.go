// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

import (
	"io"
	"sync"
	"testing"
	"time"
)

// fakePort is an in-memory io.ReadWriteCloser whose Read blocks until
// Close, mimicking an idle bill acceptor.
type fakePort struct {
	closed chan struct{}
	once   sync.Once
}

func newFakePort() *fakePort { return &fakePort{closed: make(chan struct{})} }

func (f *fakePort) Read(p []byte) (int, error) {
	<-f.closed
	return 0, io.EOF
}

func (f *fakePort) Write(p []byte) (int, error) {
	select {
	case <-f.closed:
		return 0, io.ErrClosedPipe
	default:
	}
	return len(p), nil
}

func (f *fakePort) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

// TestCloseStopsRun verifies Close interrupts a Run blocked in a read,
// rather than leaking the goroutine and file descriptor.
func TestCloseStopsRun(t *testing.T) {
	d := &Device{port: newFakePort(), state: stateWaitH1, done: make(chan struct{})}
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

// TestConcurrentRuntimeAccess drives the parser on one goroutine while
// another mutates runtime config and reads results via the accessors.
// Run with -race to prove the shared fields are properly synchronised.
func TestConcurrentRuntimeAccess(t *testing.T) {
	d := newTestDevice()
	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)

	// Parser goroutine (the role Run plays in production).
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			d.FeedByte(infoH)
			for i := 0; i < infoPayloadLen; i++ {
				d.FeedByte('A')
			}
			d.FeedByte(serialNumberH)
			for i := 0; i < serialNumberLen; i++ {
				d.FeedByte('B')
			}
		}
	}()

	// Caller goroutine: runtime setters + result accessors.
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			d.SetEscrowMode(AutoAccept)
			_ = d.EscrowMode()
			d.SetInfoCallback(func(_ *Device, _, _ string) {})
			d.SetVersionCallback(func(_ *Device, _, _, _ string) {})
			_ = d.GetModel()
			_ = d.GetManufacturer()
			_ = d.GetSerialNumber()
			_ = d.GetVersion()
			_ = d.GetCurrencyCode()
			_ = d.GetChecksum()
			_ = d.GetSensorStatus()
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}
