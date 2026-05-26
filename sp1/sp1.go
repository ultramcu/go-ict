// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"fmt"
	"io"
	"sync"
	"time"

	"go.bug.st/serial"
)

// queueCapacity is the size of the per-Device write queue. Every
// Write/WriteString call enqueues bytes here and the drainer
// goroutine pops them. 2048 is large enough for several full receipts
// to be in flight concurrently without back-pressuring the caller.
const queueCapacity = 2048

// commitGap is the minimum idle period the SP1 needs after a CR
// (commit) byte before it will reliably accept the next batch of
// bytes. Three seconds matches the reference firmware behaviour.
const commitGap = 3 * time.Second

// readChunk is the per-iteration read buffer size for Run.
const readChunk = 128

// readSleepOnError is how long Run waits before retrying after a
// serial Read error.
const readSleepOnError = 100 * time.Millisecond

// readTimeout bounds each blocking serial read so Run wakes up
// periodically to observe Close. Without it a read on an idle device
// blocks indefinitely and Close cannot stop Run, leaking the goroutine
// and the file descriptor.
const readTimeout = 200 * time.Millisecond

// drainTickInterval is the small inter-byte sleep the drainer goroutine
// uses to avoid hammering the serial port at full CPU speed when there
// is a long burst of queued bytes.
const drainTickInterval = 1 * time.Millisecond

// Device represents one SP1 attached to a serial port. Construct
// with New, start the read+drain loops with `go dev.Run()`, then
// call any of the Print*/Set*/Execute methods to drive the printer.
//
// All write entry points are safe for concurrent use; they push
// bytes into an internal queue that a single drainer goroutine
// pulls from.
type Device struct {
	// port is typed as an interface so tests can inject a fake.
	port io.ReadWriteCloser

	muWrite sync.Mutex
	muRead  sync.Mutex

	queue *byteQueue
	timer *elapsedTimer

	done      chan struct{}
	closeOnce sync.Once

	logf Logf
}

// New opens the printer on `port` at `baud` and returns a Device
// ready to start its read+drain goroutines via Run().
//
// `logf` may be nil. The returned Device is non-nil even when err
// is non-nil so the caller can inspect the configuration; Run /
// Close must not be called when err != nil.
func New(port string, baud int, logf Logf) (*Device, error) {
	d := &Device{
		queue: newByteQueue(queueCapacity),
		timer: newElapsedTimer(),
		done:  make(chan struct{}),
		logf:  logf,
	}

	p, err := serial.Open(port, &serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		d.logSP1("open %s @ %d failed: %v\r\n", port, baud, err)
		return d, err
	}
	if err := p.SetReadTimeout(readTimeout); err != nil {
		_ = p.Close()
		d.logSP1("set read timeout on %s failed: %v\r\n", port, err)
		return d, err
	}
	d.port = p
	d.log("SP1 device opened on %s @ %d\r\n", port, baud)
	return d, nil
}

// Init sends the two ESC/POS bytes that switch the printer into
// command mode and reset to factory defaults. Call it once after
// starting Run() and before issuing any print commands.
func (d *Device) Init() {
	d.ActivateESCPOSMode()
	d.PrinterResetToInitial()
}

// Run starts the printer's read loop and write drainer. It blocks
// until Close is called. Caller is expected to invoke it from its
// own goroutine: `go dev.Run()`.
func (d *Device) Run() {
	if d.port == nil {
		return
	}
	go d.drainWriteQueue()

	buf := make([]byte, readChunk)
	for {
		select {
		case <-d.done:
			return
		default:
		}

		n, err := d.read(buf)
		if err != nil {
			time.Sleep(readSleepOnError)
			continue
		}
		if d.logf != nil {
			rx := "RX:"
			for i := 0; i < n; i++ {
				rx += fmt.Sprintf(" 0x%02X", buf[i])
			}
			d.logSP1("%s\r\n", rx)
		}
	}
}

// Close releases the serial port, signals Run to return, and shuts
// down the write drainer. Safe to call multiple times.
func (d *Device) Close() error {
	var err error
	d.closeOnce.Do(func() {
		close(d.done)
		d.queue.Close()
		if d.port != nil {
			err = d.port.Close()
		}
	})
	return err
}

// drainWriteQueue is the single goroutine that owns the underlying
// serial port for writes. It pops bytes from the queue and, after
// every CR (which commits a print job on the SP1), sleeps for the
// remainder of `commitGap` before sending the next byte.
func (d *Device) drainWriteQueue() {
	owesGap := false // set after every CR until the commit gap is paid

	for {
		b, ok := d.queue.Dequeue()
		if !ok {
			return // queue closed
		}

		if owesGap {
			elapsed := d.timer.SinceLastCall()
			if elapsed < commitGap {
				wait := commitGap - elapsed
				d.log("Idling %s after CR\r\n", wait)
				time.Sleep(wait)
			}
			owesGap = false
		}

		d.hardwareWrite([]byte{b})

		if b == CR {
			// Measure the gap from the moment this CR was sent and
			// require it before the next byte -- including when the
			// next byte is itself a CR (back-to-back commits, which
			// the previous state machine failed to pace).
			_ = d.timer.SinceLastCall()
			owesGap = true
			d.log("CR sent; will idle %s before next byte\r\n", commitGap)
		}

		time.Sleep(drainTickInterval)
	}
}

// read is a mutex-protected wrapper around port.Read.
func (d *Device) read(buf []byte) (int, error) {
	d.muRead.Lock()
	defer d.muRead.Unlock()
	return d.port.Read(buf)
}

// Write enqueues each byte of data. Returns the number of bytes
// enqueued; this is always len(data) but the int return is kept for
// consistency with io.Writer-style call sites.
//
// Write does not block on serial output -- bytes are handed off to
// the drainer goroutine. It can block if the queue is full, in
// which case it back-pressures the caller until the drainer makes
// room.
func (d *Device) Write(data []byte) int {
	for i, b := range data {
		if !d.queue.Enqueue(b) {
			return i // queue closed; report how many made it in
		}
	}
	return len(data)
}

// WriteString is the string version of Write. Multi-byte UTF-8
// characters are sent verbatim; the SP1 must be configured with the
// matching code page for them to render correctly.
func (d *Device) WriteString(s string) int {
	for i := 0; i < len(s); i++ {
		if !d.queue.Enqueue(s[i]) {
			return i // queue closed; report how many made it in
		}
	}
	return len(s)
}

// hardwareWrite is the low-level serial write -- used only by the
// drainer goroutine, which already serialises calls to it.
func (d *Device) hardwareWrite(data []byte) {
	d.muWrite.Lock()
	defer d.muWrite.Unlock()
	if d.port == nil {
		return
	}
	n, err := d.port.Write(data)
	if err != nil {
		d.logSP1("serial write failed: %v\r\n", err)
		return
	}
	if n != len(data) {
		d.logSP1("serial short write: %d of %d bytes\r\n", n, len(data))
	}
}
