// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

// Each Req* method sends a single-byte request to the device.
// All return any error reported by the underlying serial write.

// ReqSerialNumber asks the device to report its 12-byte serial number.
// The result is delivered out-of-band via the device's internal state
// machine; read it from Device.SerialNumber after the response arrives.
func (d *Device) ReqSerialNumber() error {
	d.logCPU("Serial Number Request, 0x%02X\r\n", serialNumberReq)
	return d.WriteByte(serialNumberReq)
}

// ReqSensorStatus asks for the 1-byte sensor bitmap. The result is
// stored in Device.SensorStatus when the response arrives.
func (d *Device) ReqSensorStatus() error {
	d.logCPU("Sensor Status Request, 0x%02X\r\n", sensorStatusReq)
	return d.WriteByte(sensorStatusReq)
}

// ReqInfo asks for the 30-byte manufacturer / model payload. Some L77
// firmware revisions ignore this command. Results are delivered to
// the InfoCallback registered with SetInfoCallback.
func (d *Device) ReqInfo() error {
	d.logCPU("Device Info Request, 0x%02X\r\n", infoReq)
	return d.WriteByte(infoReq)
}

// ReqBillAccept tells the device to push the bill currently in escrow
// into the stacker.
func (d *Device) ReqBillAccept() error {
	d.logCPU("Bill Accept Request, 0x%02X\r\n", escrowAcceptReq)
	return d.WriteByte(escrowAcceptReq)
}

// ReqBillReject tells the device to return the bill currently in
// escrow to the user.
func (d *Device) ReqBillReject() error {
	d.logCPU("Bill Reject Request, 0x%02X\r\n", escrowRejectReq)
	return d.WriteByte(escrowRejectReq)
}

// ReqBillHold tells the device to keep the bill in escrow while the
// controller decides what to do. Useful with the AutoHold escrow mode.
func (d *Device) ReqBillHold() error {
	d.logCPU("Bill Hold Request, 0x%02X\r\n", escrowHoldReq)
	return d.WriteByte(escrowHoldReq)
}

// ReqStatus asks the device to emit a status byte. The byte is
// delivered to the StatusCallback.
func (d *Device) ReqStatus() error {
	d.logCPU("Device Status Request, 0x%02X\r\n", statusReq)
	return d.WriteByte(statusReq)
}

// ReqEnable enables bill acceptance.
func (d *Device) ReqEnable() error {
	d.logCPU("Device Enable Request, 0x%02X\r\n", enableReq)
	return d.WriteByte(enableReq)
}

// ReqDisable inhibits bill acceptance (the device will reject every
// inserted bill until ReqEnable is called).
func (d *Device) ReqDisable() error {
	d.logCPU("Device Disable Request, 0x%02X\r\n", disableReq)
	return d.WriteByte(disableReq)
}

// ReqReset performs a soft reset.
func (d *Device) ReqReset() error {
	d.logCPU("Device Reset Request, 0x%02X\r\n", resetReq)
	return d.WriteByte(resetReq)
}

// ReqVersion asks the device for its firmware version, checksum, and
// currency code. The result is delivered to the VersionCallback.
func (d *Device) ReqVersion() error {
	d.logCPU("Device Version Request, 0x%02X\r\n", versionReq)
	return d.WriteByte(versionReq)
}

// resPowerUp is the controller's mandatory reply to the 0x80 0x8F
// power-up handshake. It is sent automatically by the state machine
// and is not normally called by users.
func (d *Device) resPowerUp() error {
	d.logCPU("Power Up Response, 0x%02X\r\n", powerUpRes)
	return d.WriteByte(powerUpRes)
}

// resInfo echoes the captured info payload back to the device. Sent
// automatically by the state machine after the inbound payload has
// been collected.
func (d *Device) resInfo() error {
	d.logCPU("Information Response, 0x%02X\r\n", infoRes)
	return d.Write(append([]byte{infoRes}, d.data...))
}
