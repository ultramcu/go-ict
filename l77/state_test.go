// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

import (
	"testing"
)

// newTestDevice returns a Device with no serial port attached, suitable
// for FeedByte-driven state-machine tests.
func newTestDevice() *Device {
	return &Device{
		state: stateWaitH1,
		done:  make(chan struct{}),
	}
}

// feed pushes a sequence of bytes through the state machine in order.
func feed(d *Device, bytes ...byte) {
	for _, b := range bytes {
		d.FeedByte(b)
	}
}

// TestEscrowAutoAccept verifies the escrow header + bill type frame
// fires the escrow callback and (because the mode is AutoAccept)
// would issue an accept request. We can't observe the request here
// because there's no port, but the callback firing is what matters.
func TestEscrowAutoAccept(t *testing.T) {
	d := newTestDevice()
	d.escrowMode = AutoAccept

	var got BillType
	var fired bool
	d.escrowCallback = func(_ *Device, b BillType) {
		fired = true
		got = b
	}

	feed(d, escrowH, byte(B100))

	if !fired {
		t.Fatal("escrow callback not fired")
	}
	if got != B100 {
		t.Errorf("escrow callback got bill 0x%02X, want 0x%02X (B100)",
			byte(got), byte(B100))
	}
	if d.state != stateWaitH1 {
		t.Errorf("state after escrow = %v, want stateWaitH1", d.state)
	}
}

// TestStackThenReject verifies the controller can distinguish a
// stack and a reject, and that the bill type stays carried across
// the two-frame escrow->stack/reject sequence.
func TestStackThenReject(t *testing.T) {
	d := newTestDevice()
	d.escrowMode = DoNothing

	var (
		stacked, rejected BillType
		stackFired        bool
		rejectFired       bool
	)
	d.escrowCallback = func(_ *Device, _ BillType) {} // capture but no-op
	d.stackingCallback = func(_ *Device, b BillType) { stacked = b; stackFired = true }
	d.rejectCallback = func(_ *Device, b BillType) { rejected = b; rejectFired = true }

	// First scenario: bill arrives -> stacker.
	feed(d, escrowH, byte(B500), escrowStackH)
	if !stackFired || stacked != B500 {
		t.Fatalf("stacked: fired=%v bill=0x%02X, want fired=true bill=B500",
			stackFired, byte(stacked))
	}

	// Second scenario: bill arrives -> rejected.
	feed(d, escrowH, byte(B20), escrowRejectH)
	if !rejectFired || rejected != B20 {
		t.Fatalf("rejected: fired=%v bill=0x%02X, want fired=true bill=B20",
			rejectFired, byte(rejected))
	}
}

// TestPowerUpHandshakeResetsCleanly verifies the H1 + H2 sequence
// drops the parser back to wait-H1 even when the H2 byte arrives
// after a delay. (We can't observe the resPowerUp send because there's
// no port; we just want to make sure the state machine doesn't get
// stuck in stateWaitH2.)
func TestPowerUpHandshakeResetsCleanly(t *testing.T) {
	d := newTestDevice()
	feed(d, powerUpH1)
	if d.state != stateWaitH2 {
		t.Fatalf("after H1 state = %v, want stateWaitH2", d.state)
	}
	feed(d, powerUpH2)
	if d.state != stateWaitH1 {
		t.Fatalf("after H2 state = %v, want stateWaitH1", d.state)
	}
}

// TestPowerUpRecoversFromGarbageH2 verifies that an unexpected byte
// in stateWaitH2 doesn't strand the parser (regression for the
// original code which fell through and stayed in H2_WAITING forever).
func TestPowerUpRecoversFromGarbageH2(t *testing.T) {
	d := newTestDevice()
	feed(d, powerUpH1, 0x55) // 0x55 is not 0x8F
	if d.state != stateWaitH1 {
		t.Errorf("garbage H2 left state = %v, want stateWaitH1", d.state)
	}
}

// TestStatusCallback verifies that a status header byte fires the
// status callback with the right Status code.
func TestStatusCallback(t *testing.T) {
	d := newTestDevice()
	var got Status
	var fired bool
	d.statusCallback = func(_ *Device, s Status) { got = s; fired = true }

	feed(d, byte(StatusBillJam))

	if !fired {
		t.Fatal("status callback not fired")
	}
	if got != StatusBillJam {
		t.Errorf("status callback got 0x%02X, want 0x%02X (BillJam)",
			byte(got), byte(StatusBillJam))
	}
}

// TestSensorStatusResetsState is a regression test for the original
// code: sensor-status state previously never reset back to wait-H1
// after capturing the byte.
func TestSensorStatusResetsState(t *testing.T) {
	d := newTestDevice()
	feed(d, sensorStatusH, 0x3F) // header + bitmap byte

	if d.SensorStatus != 0x3F {
		t.Errorf("SensorStatus = 0x%02X, want 0x3F", d.SensorStatus)
	}
	if d.state != stateWaitH1 {
		t.Errorf("state after sensor read = %v, want stateWaitH1", d.state)
	}
}

// TestSerialNumberCapture verifies the 12-byte serial-number frame
// is captured correctly and returns the parser to wait-H1.
func TestSerialNumberCapture(t *testing.T) {
	d := newTestDevice()
	sn := []byte("ABCD12345678") // 12 bytes
	feed(d, serialNumberH)
	for _, b := range sn {
		d.FeedByte(b)
	}
	if d.SerialNumber != string(sn) {
		t.Errorf("SerialNumber = %q, want %q", d.SerialNumber, string(sn))
	}
	if d.state != stateWaitH1 {
		t.Errorf("state after SN = %v, want stateWaitH1", d.state)
	}
}

// TestVersionFrame walks the multi-stage version response: header +
// 19 version bytes, two checksum bytes, currency code, terminator.
func TestVersionFrame(t *testing.T) {
	d := newTestDevice()

	var (
		gotVersion, gotChecksum, gotCurrency string
		fired                                bool
	)
	d.versionCallback = func(_ *Device, v, c, cur string) {
		gotVersion, gotChecksum, gotCurrency = v, c, cur
		fired = true
	}

	// 20-char total version (1 header + 19 payload).
	versionPayload := "1.2.3-build-20260504" // 20 chars exactly
	if len(versionPayload) != 20 {
		t.Fatalf("test fixture wrong length: %d", len(versionPayload))
	}

	feed(d, versionH)         // 0x4C captured as 1st char
	for i := 1; i < 20; i++ { // remaining 19 chars
		d.FeedByte(versionPayload[i])
	}
	feed(d, 0xAB, 0xCD)               // checksum bytes
	for _, b := range []byte("THB") { // currency code
		d.FeedByte(b)
	}
	d.FeedByte(currencyTerminator) // '#'

	if !fired {
		t.Fatal("version callback not fired")
	}

	wantVersion := string(versionH) + versionPayload[1:]
	if gotVersion != wantVersion {
		t.Errorf("version = %q, want %q", gotVersion, wantVersion)
	}
	if gotChecksum != "\xAB\xCD" {
		t.Errorf("checksum = %x, want %x", []byte(gotChecksum), []byte{0xAB, 0xCD})
	}
	if gotCurrency != "THB" {
		t.Errorf("currency = %q, want %q", gotCurrency, "THB")
	}
}

// TestAutoHoldSendsHoldImmediately is a smoke test of the AutoHold
// branch: when escrow arrives, the parser tries to send Hold via
// WriteByte. Without a port WriteByte returns an error which we
// just ignore -- the point of the test is that the escrow callback
// still fires and the state ends back at wait-H1.
func TestAutoHoldSendsHoldImmediately(t *testing.T) {
	d := newTestDevice()
	d.escrowMode = AutoHold

	var fired bool
	d.escrowCallback = func(_ *Device, _ BillType) { fired = true }

	feed(d, escrowH, byte(B1000))
	if !fired {
		t.Fatal("escrow callback not fired in AutoHold mode")
	}
	if d.state != stateWaitH1 {
		t.Errorf("state after AutoHold escrow = %v, want stateWaitH1", d.state)
	}
}

func TestBillTypeValueAndString(t *testing.T) {
	cases := []struct {
		b    BillType
		val  int
		text string
	}{
		{B20, 20, "20"},
		{B50, 50, "50"},
		{B100, 100, "100"},
		{B500, 500, "500"},
		{B1000, 1000, "1000"},
		{BillType(0xFF), 0, "Unknown"},
	}
	for _, tc := range cases {
		if got := tc.b.Value(); got != tc.val {
			t.Errorf("(0x%02X).Value() = %d, want %d", byte(tc.b), got, tc.val)
		}
		if got := tc.b.String(); got != tc.text {
			t.Errorf("(0x%02X).String() = %q, want %q", byte(tc.b), got, tc.text)
		}
	}
}

func TestEscrowModeString(t *testing.T) {
	cases := []struct {
		m    EscrowMode
		want string
	}{
		{DoNothing, "DoNothing"},
		{AutoAccept, "AutoAccept"},
		{AutoReject, "AutoReject"},
		{AutoHold, "AutoHold"},
		{EscrowMode(99), "Unknown"},
	}
	for _, tc := range cases {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("(%d).String() = %q, want %q", int(tc.m), got, tc.want)
		}
	}
}
