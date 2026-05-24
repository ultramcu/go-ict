// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

import "testing"

func TestInfoFrame(t *testing.T) {
	d := newTestDevice()
	var gotModel, gotMfg string
	var fired bool
	d.infoCallback = func(_ *Device, m, mf string) { gotModel, gotMfg, fired = m, mf, true }

	payload := make([]byte, infoPayloadLen)
	copy(payload, "MODEL01")   // bytes 0..6
	payload[7] = ' '           // separator
	copy(payload[8:], "MFG99") // bytes 8..12

	d.FeedByte(infoH)
	feed(d, payload...)

	if !fired {
		t.Fatal("info callback not fired")
	}
	if gotModel != "MODEL01" || gotMfg != "MFG99" {
		t.Errorf("info = (%q,%q), want (MODEL01,MFG99)", gotModel, gotMfg)
	}
	if d.GetModel() != "MODEL01" || d.GetManufacturer() != "MFG99" {
		t.Errorf("accessors = (%q,%q), want (MODEL01,MFG99)", d.GetModel(), d.GetManufacturer())
	}
	if d.state != stateWaitH1 {
		t.Errorf("state after info = %v, want stateWaitH1", d.state)
	}
}

func TestSerialNumberFrame(t *testing.T) {
	d := newTestDevice()
	d.FeedByte(serialNumberH)
	feed(d, []byte("SN0123456789")...) // 12 bytes

	if got := d.GetSerialNumber(); got != "SN0123456789" {
		t.Errorf("serial = %q, want SN0123456789", got)
	}
	if d.state != stateWaitH1 {
		t.Errorf("state = %v, want stateWaitH1", d.state)
	}
}

func TestVersionChecksumCurrencyFrame(t *testing.T) {
	d := newTestDevice()
	var v, cs, cc string
	var fired bool
	d.versionCallback = func(_ *Device, version, checksum, currency string) {
		v, cs, cc, fired = version, checksum, currency, true
	}

	d.FeedByte(versionH) // 0x4C, first byte of the version string
	for i := 0; i < versionPayloadLen; i++ {
		d.FeedByte('V')
	}
	d.FeedByte(0xAB) // checksum[0]
	d.FeedByte(0xCD) // checksum[1]
	feed(d, 'T', 'H', 'B', currencyTerminator)

	if !fired {
		t.Fatal("version callback not fired")
	}
	wantVer := "L" + "VVVVVVVVVVVVVVVVVVV" // 0x4C='L' + 19 'V'
	if v != wantVer {
		t.Errorf("version = %q, want %q", v, wantVer)
	}
	if cs != string([]byte{0xAB, 0xCD}) {
		t.Errorf("checksum = % X, want AB CD", []byte(cs))
	}
	if cc != "THB" {
		t.Errorf("currency = %q, want THB", cc)
	}
	if got := d.GetChecksum(); got != [2]byte{0xAB, 0xCD} {
		t.Errorf("GetChecksum = % X, want AB CD", got)
	}
	if d.GetVersion() != wantVer || d.GetCurrencyCode() != "THB" {
		t.Errorf("accessors mismatch: %q / %q", d.GetVersion(), d.GetCurrencyCode())
	}
}

// TestCurrencyCapNoTerminator guards the unbounded-buffer hardening:
// a currency field that never terminates is dropped, not grown forever.
func TestCurrencyCapNoTerminator(t *testing.T) {
	d := newTestDevice()
	fired := false
	d.versionCallback = func(_ *Device, _, _, _ string) { fired = true }

	// Reach the currency-collecting state.
	d.FeedByte(versionH)
	for i := 0; i < versionPayloadLen; i++ {
		d.FeedByte('V')
	}
	d.FeedByte(0)
	d.FeedByte(0)

	// Feed more than the cap with no '#': must reset without firing.
	for i := 0; i < maxCurrencyCodeLen+8; i++ {
		d.FeedByte('X')
	}
	if fired {
		t.Error("version callback fired on an unterminated currency field")
	}
	if d.state != stateWaitH1 {
		t.Errorf("state = %v, want stateWaitH1 after cap reset", d.state)
	}
}

func TestSensorStatusFrame(t *testing.T) {
	d := newTestDevice()
	d.FeedByte(sensorStatusH)
	d.FeedByte(0x2A)
	if got := d.GetSensorStatus(); got != 0x2A {
		t.Errorf("sensor status = 0x%02X, want 0x2A", got)
	}
	if d.state != stateWaitH1 {
		t.Errorf("state = %v, want stateWaitH1", d.state)
	}
}

func TestStatusCallbackFires(t *testing.T) {
	d := newTestDevice()
	var got Status
	var fired bool
	d.statusCallback = func(_ *Device, s Status) { got, fired = s, true }

	d.FeedByte(byte(StatusBillJam))
	if !fired || got != StatusBillJam {
		t.Errorf("status callback got (%v, fired=%v), want (BillJam, true)", got, fired)
	}
}

func TestStatusStringAll(t *testing.T) {
	codes := []Status{
		StatusMotorFailure, StatusChecksumError, StatusBillJam, StatusBillRemove,
		StatusStackerOpen, StatusSensorProblem, StatusBillFish, StatusStackerProblem,
		StatusBillReject, StatusInvalidCommand, StatusReserved, StatusErrorExclusion,
		StatusEnabled, StatusInhibited,
	}
	for _, c := range codes {
		if c.String() == "Unknown" {
			t.Errorf("status 0x%02X rendered as Unknown", byte(c))
		}
	}
	if Status(0x00).String() != "Unknown" {
		t.Error("unmapped status should render Unknown")
	}
}
