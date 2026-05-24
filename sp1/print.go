// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"fmt"
	"time"
)

// SetFontSize changes the active glyph size for subsequent text.
// Sends ESC M {0,1,2}.
func (d *Device) SetFontSize(size FontSize) {
	d.logCPU("SetFontSize %s\r\n", size.String())
	switch size {
	case Small:
		d.WriteString("\x1B\x4D\x30")
	case Large:
		d.WriteString("\x1B\x4D\x32")
	default: // Normal and unknown sizes both fall back to Normal.
		d.WriteString("\x1B\x4D\x31")
	}
}

// SetUnderline toggles underlined output. Sends ESC - {0,1}.
func (d *Device) SetUnderline(on bool) {
	val := byte('0')
	label := "Disable"
	if on {
		val = '1'
		label = "Enable"
	}
	d.logCPU("SetUnderline %s\r\n", label)
	d.Write([]byte{ESC, '-', val})
}

// ActivateESCPOSMode switches the printer into ESC/POS command mode.
// Required as the very first command after power-on.
func (d *Device) ActivateESCPOSMode() {
	d.logCPU("Activate ESC/POS Mode\r\n")
	d.Write([]byte{ESC, '#'})
}

// PrinterResetToInitial restores factory-default settings. Sends
// ESC @.
func (d *Device) PrinterResetToInitial() {
	d.logCPU("Printer Reset To Initial\r\n")
	d.Write([]byte{ESC, '@'})
}

// CharactersHighlight toggles highlighted (inverted) output. Sends
// GS B {0,1}.
func (d *Device) CharactersHighlight(on bool) {
	val := byte('0')
	label := "Disable"
	if on {
		val = '1'
		label = "Enable"
	}
	d.logCPU("Characters Highlight %s\r\n", label)
	d.Write([]byte{GS, 'B', val})
}

// TurnOffCommunicationDataAutoClearMode disables the printer's
// behaviour of clearing the input buffer on certain status
// transitions. Sends ESC c 5.
func (d *Device) TurnOffCommunicationDataAutoClearMode() {
	d.logCPU("Turn Off Communication-Data Auto-Clear Mode\r\n")
	d.Write([]byte{ESC, 'c', '5'})
}

// Execute commits the current print job by sending CR. The drainer
// will idle for `commitGap` before sending the next byte to give the
// printer time to render and feed paper.
func (d *Device) Execute() {
	d.logCPU("Execute\r\n")
	d.Write([]byte{CR})
}

// LineFeed sends a single newline.
func (d *Device) LineFeed() {
	d.logCPU("LF\r\n")
	d.WriteString("\n")
}

// Printf is fmt.Sprintf piped to the printer.
func (d *Device) Printf(format string, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	d.logCPU("Print: %s\r\n", RemoveNewline(text))
	d.WriteString(text)
}

// PrintfFormat sets the font size, software-aligns the text inside
// the per-font line budget, and sends it. Convenient for centred
// titles and right-aligned totals.
func (d *Device) PrintfFormat(size FontSize, align Alignment, format string, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	d.logCPU("Print(format): %s, FontSize %s, Alignment %s\r\n",
		RemoveNewline(text), size.String(), align.String())
	d.SetFontSize(size)
	d.WriteString(AlignmentTextWithSpace(size, align, text))
}

// SetQRCodeSize sets the QR module size (1..7). Larger = bigger code.
// Sends GS ( k 03 00 31 43 n.
//
// The 1-second sleep at the end gives the printer's QR engine time
// to ingest the parameter before the next command lands -- empirical
// requirement from the SP1 firmware.
func (d *Device) SetQRCodeSize(size int) {
	if size < 1 {
		size = 1
	}
	if size > 7 {
		d.log("QR Code size %d clamped to 7\r\n", size)
		size = 7
	}
	d.logCPU("Set QR Code Size %d\r\n", size)
	d.Write([]byte{GS, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, byte(size)})
	time.Sleep(1 * time.Second)
}

// GenerateQRCode emits a QR code containing `data`. Use SetQRCodeSize
// first to control the module size.
func (d *Device) GenerateQRCode(data string) {
	if len(data) > 0xFFFF {
		d.log("QR data %d bytes exceeds 65535; truncating\r\n", len(data))
		data = data[:0xFFFF]
	}
	lenLo := byte(len(data) % 256) // low byte of the length
	lenHi := byte(len(data) / 256) // high byte of the length
	d.logCPU("Generate QR Code: %s\r\n", RemoveNewline(data))
	d.Write([]byte{GS, 'k', QRCode, lenLo, lenHi})
	d.WriteString(data)
}

// GetPrinterSystemInformation requests the printer's identification
// payload. Reply bytes arrive on the read channel; subscribe by
// hooking SetLogFunc and watching for the response, or extend Run
// to dispatch them.
func (d *Device) GetPrinterSystemInformation() {
	d.logCPU("Get Printer System Information\r\n")
	d.Write([]byte{ESC, 0x28, 'T'})
}
