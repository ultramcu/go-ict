// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

// Logf matches the signature of log.Printf and friends. Pass nil to
// New() (or SetLogFunc) to disable logging.
type Logf func(format string, args ...interface{}) string

// EscrowCallback fires whenever a bill enters/leaves the escrow path
// (escrow accepted, stacked, or rejected, depending on which slot
// the callback was passed to).
type EscrowCallback func(d *Device, bill BillType)

// StatusCallback fires for every status frame the device emits.
type StatusCallback func(d *Device, status Status)

// InfoCallback fires once per ReqInfo round-trip with the model and
// manufacturer strings the device reports.
type InfoCallback func(d *Device, model, manufacturer string)

// VersionCallback fires once per version-frame round-trip with the
// firmware version, 2-byte checksum (as a string), and currency code.
type VersionCallback func(d *Device, version, checksum, currencyCode string)

// EscrowMode controls the parser's reaction when a bill arrives in
// escrow. The four values are mutually exclusive.
type EscrowMode int

const (
	// DoNothing leaves the decision to the caller's escrow callback;
	// the caller must invoke ReqBillAccept / ReqBillReject / ReqBillHold
	// to move the bill out of escrow.
	DoNothing EscrowMode = iota
	// AutoAccept sends ReqBillAccept automatically after the escrow
	// callback returns.
	AutoAccept
	// AutoReject sends ReqBillReject automatically after the escrow
	// callback returns.
	AutoReject
	// AutoHold sends ReqBillHold immediately when the bill enters
	// escrow (before the escrow callback runs); the callback is then
	// expected to decide and call Accept/Reject.
	AutoHold
)

// String renders an EscrowMode for logging.
func (e EscrowMode) String() string {
	switch e {
	case DoNothing:
		return "DoNothing"
	case AutoAccept:
		return "AutoAccept"
	case AutoReject:
		return "AutoReject"
	case AutoHold:
		return "AutoHold"
	default:
		return "Unknown"
	}
}

// BillType is the protocol byte the device uses to identify a
// denomination. Value() returns the human-readable face value.
type BillType byte

const (
	B20   BillType = 0x01
	B50   BillType = 0x05
	B100  BillType = 0x09
	B500  BillType = 0x0D
	B1000 BillType = 0x11
)

// Value returns the denomination's face value (0 for an unknown
// bill code).
func (b BillType) Value() int {
	switch b {
	case B20:
		return 20
	case B50:
		return 50
	case B100:
		return 100
	case B500:
		return 500
	case B1000:
		return 1000
	default:
		return 0
	}
}

// String renders a BillType as its face value.
func (b BillType) String() string {
	switch b {
	case B20:
		return "20"
	case B50:
		return "50"
	case B100:
		return "100"
	case B500:
		return "500"
	case B1000:
		return "1000"
	default:
		return "Unknown"
	}
}

// Status is one of the status / error byte codes the device emits
// in response to a status request, or unsolicited when an error
// occurs.
type Status byte

// Value returns the raw status byte.
func (s Status) Value() byte { return byte(s) }

// String renders a Status as a short human label.
func (s Status) String() string {
	switch s {
	case StatusMotorFailure:
		return "Motor Failure"
	case StatusChecksumError:
		return "Checksum Error"
	case StatusBillJam:
		return "Bill Jam"
	case StatusBillRemove:
		return "Bill Remove"
	case StatusStackerOpen:
		return "Stacker Open"
	case StatusSensorProblem:
		return "Sensor Problem"
	case StatusBillFish:
		return "Bill Fish"
	case StatusStackerProblem:
		return "Stacker Problem"
	case StatusBillReject:
		return "Bill Reject"
	case StatusInvalidCommand:
		return "Invalid Command"
	case StatusReserved:
		return "Reserved"
	case StatusErrorExclusion:
		return "Error Exclusion"
	case StatusEnabled:
		return "Bill Acceptor Enabled"
	case StatusInhibited:
		return "Bill Acceptor Inhibited"
	default:
		return "Unknown"
	}
}

// state is the parser's internal cursor. It is unexported because
// callers interact with the device only via callbacks; they never
// inspect the state machine directly.
type state int

const (
	stateWaitH1 state = iota
	stateWaitH2
	stateGetBillType
	stateInfoGetting
	stateVersionGetting
	stateChecksum1Getting
	stateChecksum2Getting
	stateCurrencyCodeGetting
	stateSerialNumberGetting
	stateSensorStatusWaiting
)
