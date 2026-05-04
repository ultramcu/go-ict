// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

// Suffix conventions used in this file:
//
//   * H   -- byte the L77 sends to the controller (header / event).
//   * Req -- byte the controller sends to the L77 (request).
//   * Res -- byte the controller sends to the L77 in response to an event.
//
// Multi-byte payloads are described in the comment block above each
// constant group.

const (
	// Power-up handshake.
	//
	// L77        Controller
	// 0x80 0x8F ->
	//                   <- 0x02 (must be sent within 2 seconds)
	powerUpH1  byte = 0x80
	powerUpH2  byte = 0x8F
	powerUpRes byte = 0x02

	// Manufacturer / model information.
	//
	// Some L77 firmware revisions do not implement this command.
	//
	// L77        Controller
	//                   <- 0x5B  (request)
	// 0x5B [DATA:30] ->        (header + 30 payload bytes)
	//                   <- 0x5C [DATA:30] (response, echoed back)
	infoReq byte = 0x5B
	infoH   byte = 0x5B
	infoRes byte = 0x5C

	// Escrow flow.
	//
	// L77                                       Controller
	// 0x81 ->                                  Bill detected; controller
	//                                          must respond within 5s or
	//                                          the bill is auto-rejected.
	// 0x40-0x59 -> (bill type byte)
	//                                    <-    0x02  Accept (stack the bill)
	//                                    <-    0x0F  Reject (return to user)
	//                                    <-    0x18  Hold in escrow
	// 0x10 -> (bill stacked)
	// 0x11 -> (bill rejected)
	escrowH         byte = 0x81
	escrowAcceptReq byte = 0x02
	escrowRejectReq byte = 0x0F
	escrowHoldReq   byte = 0x18
	escrowStackH    byte = 0x10
	escrowRejectH   byte = 0x11

	// Status polling.
	//
	// L77                  Controller
	//                <-     0x0C   (request)
	// 0x20..0x5E ->         (status byte; see Status* constants below)
	statusReq byte = 0x0C

	// Enable / disable / reset.
	enableReq  byte = 0x3E
	disableReq byte = 0x5E
	resetReq   byte = 0x30

	// Version / checksum / currency code.
	//
	// L77                                Controller
	//                                <-  0x5A
	// 0x4C [VERSION:19] [CHECKSUM:2] [CURRENCY] '#' ->
	//
	// The version byte 0x4C is included as the first byte of the
	// 20-byte version string. The currency code is variable-length
	// and terminated by the ASCII '#' byte.
	versionReq byte = 0x5A
	versionH   byte = 0x4C

	// DIP-switch read (currently unused by this package; kept for
	// completeness so callers can issue raw writes if needed).
	dipSwitchReq byte = 0x5D

	// Serial number.
	//
	// L77                       Controller
	//                       <-  0x5F
	// 0x5F [SERIAL:12]    ->
	serialNumberReq byte = 0x5F
	serialNumberH   byte = 0x5F

	// Sensor status. The 1-byte payload is a bitmap; bit definitions:
	//   bit 0  Input sensor   (0 OK, 1 fault)
	//   bit 1  Middle sensor
	//   bit 2  Output sensor
	//   bit 3  Hook sensor
	//   bit 4  Fish sensor
	//   bit 5  Stacker sensor
	//   bit 6,7 reserved
	sensorStatusReq byte = 0x60
	sensorStatusH   byte = 0x60

	// Motor direction.
	motorDirReq byte = 0x61
	motorDirH   byte = 0x61
)

// Status byte codes returned by the device. These are exported so
// callers can compare against the StatusCallback's argument.
const (
	StatusMotorFailure   Status = 0x20
	StatusChecksumError  Status = 0x21
	StatusBillJam        Status = 0x22
	StatusBillRemove     Status = 0x23
	StatusStackerOpen    Status = 0x24
	StatusSensorProblem  Status = 0x25
	StatusBillFish       Status = 0x27
	StatusStackerProblem Status = 0x28
	StatusBillReject     Status = 0x29
	StatusInvalidCommand Status = 0x2A
	StatusReserved       Status = 0x2E
	StatusErrorExclusion Status = 0x2F
	StatusEnabled        Status = 0x3E
	StatusInhibited      Status = 0x5E
)

// Payload-length constants used by the state machine.
const (
	infoPayloadLen     = 30 // bytes after the 0x5B header
	versionPayloadLen  = 19 // bytes after the 0x4C header (total version string is 20)
	serialNumberLen    = 12
	currencyTerminator = '#'
)
