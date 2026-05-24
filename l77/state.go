// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

// FeedByte advances the state machine by one byte. It is exported
// primarily for unit testing -- normal use is via Run(), which reads
// from the serial port and calls FeedByte for each byte received.
//
// FeedByte may invoke any of the registered callbacks before
// returning. It is not safe for concurrent use; callers must
// serialise access (Run does this naturally as a single goroutine).
func (d *Device) FeedByte(b byte) {
	switch d.state {
	case stateWaitH1:
		d.handleStateWaitH1(b)
	case stateWaitH2:
		d.handleStateWaitH2(b)
	case stateGetBillType:
		d.handleStateGetBillType(b)
	case stateInfoGetting:
		d.handleStateInfoGetting(b)
	case stateVersionGetting:
		d.handleStateVersionGetting(b)
	case stateChecksum1Getting:
		d.mu.Lock()
		d.Checksum[0] = b
		d.mu.Unlock()
		d.state = stateChecksum2Getting
	case stateChecksum2Getting:
		d.mu.Lock()
		d.Checksum[1] = b
		d.mu.Unlock()
		d.state = stateCurrencyCodeGetting
	case stateCurrencyCodeGetting:
		d.handleStateCurrencyCodeGetting(b)
	case stateSerialNumberGetting:
		d.handleStateSerialNumberGetting(b)
	case stateSensorStatusWaiting:
		d.mu.Lock()
		d.SensorStatus = b
		d.mu.Unlock()
		d.stateReset()
	default:
		// Unknown state -- defensively reset.
		d.stateReset()
	}
}

// stateReset clears the per-message state. The Checksum buffer is
// cleared too; callers that need to retain it should snapshot it
// from the VersionCallback first. The currentBillType is preserved
// across stateReset because the device emits the bill type in the
// escrow frame and we need it later for the matching stack/reject
// frame.
func (d *Device) stateReset() {
	d.state = stateWaitH1
	d.data = d.data[:0]
	d.dataCnt = 0
}

// handleStateWaitH1 dispatches the first byte of every device frame.
func (d *Device) handleStateWaitH1(b byte) {
	switch b {

	case powerUpH1:
		d.logNewLine()
		d.logL77("Power Up H1, 0x%02X\r\n", b)
		d.state = stateWaitH2

	case escrowH:
		d.state = stateGetBillType

	case infoH:
		d.state = stateInfoGetting

	case escrowStackH:
		bill := d.currentBillType
		d.logL77("Bill Stacked, Type 0x%02X (%d)\r\n", byte(bill), bill.Value())
		d.logMoney("%d\r\n", bill.Value())
		if d.stackingCallback != nil {
			d.stackingCallback(d, bill)
		}
		d.currentBillType = 0
		d.stateReset()

	case escrowRejectH:
		bill := d.currentBillType
		d.logL77("Bill Rejected, Type 0x%02X (%d)\r\n", byte(bill), bill.Value())
		if d.rejectCallback != nil {
			d.rejectCallback(d, bill)
		}
		d.currentBillType = 0
		d.stateReset()

	case versionH:
		// 0x4C is both the trigger and the first byte of the version
		// string the spec gives back, so capture it in the buffer.
		d.data = append(d.data, b)
		d.state = stateVersionGetting

	case serialNumberH:
		d.state = stateSerialNumberGetting

	case sensorStatusH:
		d.state = stateSensorStatusWaiting

	case byte(StatusMotorFailure),
		byte(StatusChecksumError),
		byte(StatusBillJam),
		byte(StatusBillRemove),
		byte(StatusStackerOpen),
		byte(StatusSensorProblem),
		byte(StatusBillFish),
		byte(StatusStackerProblem),
		byte(StatusBillReject),
		byte(StatusInvalidCommand),
		byte(StatusErrorExclusion),
		byte(StatusEnabled),
		byte(StatusInhibited):
		s := Status(b)
		d.logL77("Status 0x%02X, %s\r\n", b, s.String())
		if d.statusCallback != nil {
			d.statusCallback(d, s)
		}
		d.stateReset()

	default:
		// Unknown header byte -- ignore and stay in wait-H1.
	}
}

// handleStateWaitH2 expects 0x8F as the second half of the power-up
// handshake. Anything else means the bus desynced; reset.
func (d *Device) handleStateWaitH2(b byte) {
	if b == powerUpH2 {
		d.logL77("Power Up H2, 0x%02X\r\n", b)
		_ = d.resPowerUp()
	}
	d.stateReset()
}

// handleStateGetBillType captures the denomination byte that follows
// the escrow header.
func (d *Device) handleStateGetBillType(b byte) {
	d.currentBillType = BillType(b)
	bill := d.currentBillType
	d.mu.RLock()
	mode := d.escrowMode
	d.mu.RUnlock()
	d.logL77("Bill Escrow, Type 0x%02X (%d), Mode %s\r\n",
		b, bill.Value(), mode.String())

	// AutoHold sends Hold immediately so the bill stays in escrow
	// while the callback decides; the caller is then expected to
	// invoke ReqBillAccept / ReqBillReject.
	if mode == AutoHold {
		_ = d.ReqBillHold()
	}

	if d.escrowCallback != nil {
		d.escrowCallback(d, bill)
	}

	switch mode {
	case AutoAccept:
		_ = d.ReqBillAccept()
	case AutoReject:
		_ = d.ReqBillReject()
	}

	d.stateReset()
}

// handleStateInfoGetting collects the 30-byte info payload then
// echoes it back to the device and fires the InfoCallback.
func (d *Device) handleStateInfoGetting(b byte) {
	d.data = append(d.data, b)
	d.dataCnt++
	if d.dataCnt < infoPayloadLen {
		return
	}
	_ = d.resInfo()
	// Per ICT documentation the payload is laid out as:
	//   bytes  0..6  (7 chars) : model
	//   byte   7     ' '       : separator
	//   bytes  8..12 (5 chars) : manufacturer
	d.mu.Lock()
	if len(d.data) >= 13 {
		d.Model = string(d.data[0:7])
		d.Manufacturer = string(d.data[8:13])
	}
	model, manuf := d.Model, d.Manufacturer
	cb := d.infoCallback
	d.mu.Unlock()
	if cb != nil {
		cb(d, model, manuf)
	}
	d.stateReset()
}

// handleStateVersionGetting collects the version bytes (header + 19)
// and transitions to checksum capture.
func (d *Device) handleStateVersionGetting(b byte) {
	d.data = append(d.data, b)
	d.dataCnt++
	if d.dataCnt < versionPayloadLen {
		return
	}
	d.mu.Lock()
	d.Version = string(d.data)
	d.mu.Unlock()
	d.data = d.data[:0]
	d.dataCnt = 0
	d.state = stateChecksum1Getting
}

// maxCurrencyCodeLen bounds the '#'-terminated currency field. A real
// currency code is only a few bytes; if the terminator is lost on a
// noisy line we drop the frame rather than letting the buffer grow
// without bound.
const maxCurrencyCodeLen = 16

// handleStateCurrencyCodeGetting collects bytes until the '#'
// terminator, then fires the VersionCallback.
func (d *Device) handleStateCurrencyCodeGetting(b byte) {
	if b == currencyTerminator {
		d.mu.Lock()
		d.CurrencyCode = string(d.data)
		version, checksum, currency := d.Version, string(d.Checksum[:]), d.CurrencyCode
		cb := d.versionCallback
		d.mu.Unlock()
		if cb != nil {
			cb(d, version, checksum, currency)
		}
		d.stateReset()
		return
	}
	if len(d.data) >= maxCurrencyCodeLen {
		d.logL77("currency code exceeded %d bytes without terminator; resetting\r\n",
			maxCurrencyCodeLen)
		d.stateReset()
		return
	}
	d.data = append(d.data, b)
}

// handleStateSerialNumberGetting collects the 12-byte serial number.
func (d *Device) handleStateSerialNumberGetting(b byte) {
	d.data = append(d.data, b)
	d.dataCnt++
	if d.dataCnt < serialNumberLen {
		return
	}
	d.mu.Lock()
	d.SerialNumber = string(d.data)
	d.mu.Unlock()
	d.stateReset()
}
