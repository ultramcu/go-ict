// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

import (
	"fmt"
	"sync"
	"time"

	"github.com/tarm/serial"
)

// defaultBaud is the L77's documented serial baud rate. It is fixed
// in firmware on every L77 unit shipped to date; use New to accept
// a port path with this baud, or NewWithBaud to override.
const defaultBaud = 9600

// readChunk is the size of the per-iteration read buffer in Run. It
// just needs to be big enough that a typical USB serial chunk fits;
// 128 is the size used by the original ICT reference code.
const readChunk = 128

// readSleepOnError is how long Run sleeps before retrying after a
// serial Read returns an error (typically a benign "device temporarily
// unavailable" while no bytes are buffered).
const readSleepOnError = 100 * time.Millisecond

// Device represents one L77 attached to a serial port.
//
// Construct with New (or NewWithBaud), wire up callbacks, then start
// the read loop with `go dev.Run()`. Stop with Close, which both
// closes the serial port and signals Run to return.
type Device struct {
	// Configuration captured at New time; not mutated after.
	cfg serial.Config

	// Underlying serial port. nil if the port could not be opened
	// (in which case New also returns the error).
	port *serial.Port

	// Public, read-after-callback fields populated by the state
	// machine. Callers should treat them as read-only and snapshot
	// any values they intend to outlive the next protocol exchange.
	Model        string
	Manufacturer string
	SerialNumber string
	Version      string
	Checksum     [2]byte
	CurrencyCode string
	SensorStatus byte

	// State-machine internals.
	state           state
	data            []byte
	dataCnt         int64
	currentBillType BillType

	// Callbacks. Any of these may be nil; FeedByte / Run guard each
	// one before invoking.
	escrowCallback   EscrowCallback
	stackingCallback EscrowCallback
	rejectCallback   EscrowCallback
	statusCallback   StatusCallback
	infoCallback     InfoCallback
	versionCallback  VersionCallback

	escrowMode EscrowMode

	// Concurrency control. Read and write to the serial port are
	// serialised; Close uses done+closeOnce to interrupt Run cleanly.
	muWrite   sync.Mutex
	muRead    sync.Mutex
	done      chan struct{}
	closeOnce sync.Once

	logf Logf
}

// New opens the L77 on `port` at 9600 baud and returns a configured
// device ready to start its read loop.
//
// All callbacks may be nil; pass nil for any you don't care about.
// The returned Device is non-nil even when err != nil so the caller
// can still inspect its configuration / log handle, but Run / Close
// must not be called when err != nil.
func New(
	port string,
	escrowCb, stackingCb, rejectCb EscrowCallback,
	statusCb StatusCallback,
	mode EscrowMode,
	logf Logf,
) (*Device, error) {
	return NewWithBaud(port, defaultBaud,
		escrowCb, stackingCb, rejectCb, statusCb, mode, logf)
}

// NewWithBaud is identical to New but accepts a non-default baud rate.
// Few real L77 units use anything other than 9600; this exists for
// adapters and bench rigs.
func NewWithBaud(
	port string, baud int,
	escrowCb, stackingCb, rejectCb EscrowCallback,
	statusCb StatusCallback,
	mode EscrowMode,
	logf Logf,
) (*Device, error) {
	d := &Device{
		cfg:              serial.Config{Name: port, Baud: baud},
		escrowCallback:   escrowCb,
		stackingCallback: stackingCb,
		rejectCallback:   rejectCb,
		statusCallback:   statusCb,
		escrowMode:       mode,
		logf:             logf,
		state:            stateWaitH1,
		done:             make(chan struct{}),
	}

	p, err := serial.OpenPort(&d.cfg)
	if err != nil {
		d.logL77("OpenPort %s @ %d failed: %v\r\n", port, baud, err)
		return d, err
	}
	d.port = p

	d.logNewLine()
	d.log("L77 device opened on %s @ %d, escrow mode %s\r\n",
		port, baud, mode.String())
	return d, nil
}

// SetEscrowMode changes the escrow handling policy at runtime.
func (d *Device) SetEscrowMode(mode EscrowMode) {
	d.escrowMode = mode
}

// EscrowMode returns the current escrow handling policy.
func (d *Device) EscrowMode() EscrowMode {
	return d.escrowMode
}

// SetInfoCallback installs / replaces the info-frame callback.
// Pass nil to disable.
func (d *Device) SetInfoCallback(cb InfoCallback) {
	d.infoCallback = cb
}

// SetVersionCallback installs / replaces the version-frame callback.
// Pass nil to disable.
func (d *Device) SetVersionCallback(cb VersionCallback) {
	d.versionCallback = cb
}

// Run reads from the serial port and feeds every received byte
// through the state machine. It returns when Close is called or
// when an unrecoverable serial error occurs. Caller is expected to
// invoke it from its own goroutine: `go dev.Run()`.
func (d *Device) Run() {
	if d.port == nil {
		return
	}
	buf := make([]byte, readChunk)
	for {
		select {
		case <-d.done:
			return
		default:
		}

		n, err := d.read(buf)
		if err != nil {
			// Read errors are common (no bytes available, port
			// momentarily unavailable). Sleep briefly so we don't
			// spin on a closed or broken port; Close will trip the
			// done channel above on the next iteration.
			time.Sleep(readSleepOnError)
			continue
		}
		for i := 0; i < n; i++ {
			d.FeedByte(buf[i])
		}
	}
}

// Close releases the serial port and signals Run to return. Safe to
// call multiple times.
func (d *Device) Close() error {
	var err error
	d.closeOnce.Do(func() {
		close(d.done)
		if d.port != nil {
			err = d.port.Close()
		}
	})
	return err
}

// read is a mutex-protected wrapper around port.Read.
func (d *Device) read(buf []byte) (int, error) {
	d.muRead.Lock()
	defer d.muRead.Unlock()
	return d.port.Read(buf)
}

// WriteByte sends one byte to the device, returning a non-nil error
// if the write fails or returns zero bytes.
func (d *Device) WriteByte(b byte) error {
	d.muWrite.Lock()
	defer d.muWrite.Unlock()
	if d.port == nil {
		return fmt.Errorf("l77: port not open")
	}
	n, err := d.port.Write([]byte{b})
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("l77: short write (1 byte requested, %d written)", n)
	}
	return nil
}

// Write sends a multi-byte payload. Returns a non-nil error if the
// underlying write fails or returns fewer bytes than requested.
func (d *Device) Write(data []byte) error {
	d.muWrite.Lock()
	defer d.muWrite.Unlock()
	if d.port == nil {
		return fmt.Errorf("l77: port not open")
	}
	n, err := d.port.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("l77: short write (%d requested, %d written)", len(data), n)
	}
	return nil
}
