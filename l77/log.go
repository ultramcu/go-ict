// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

import "fmt"

// SetLogFunc replaces the device's log function. Pass nil to disable
// logging.
func (d *Device) SetLogFunc(f Logf) {
	d.logf = f
}

// logNewLine writes a bare newline through the log function (if set).
func (d *Device) logNewLine() {
	if d.logf != nil {
		d.logf("\r\n")
	}
}

// log writes a generic line tagged "[   ]".
func (d *Device) log(format string, args ...interface{}) {
	if d.logf == nil {
		return
	}
	d.logf("[   ] : %s", fmt.Sprintf(format, args...))
}

// logL77 writes a line tagged "[L77]" — bytes received from the device.
func (d *Device) logL77(format string, args ...interface{}) {
	if d.logf == nil {
		return
	}
	d.logf("[L77] : %s", fmt.Sprintf(format, args...))
}

// logCPU writes a line tagged "[CPU]" — bytes the controller sent to
// the device.
func (d *Device) logCPU(format string, args ...interface{}) {
	if d.logf == nil {
		return
	}
	d.logf("[CPU] : %s", fmt.Sprintf(format, args...))
}

// logMoney writes a line tagged "[MNY]" — used for accounting events
// like a successful stack of a recognised denomination.
func (d *Device) logMoney(format string, args ...interface{}) {
	if d.logf == nil {
		return
	}
	d.logf("[MNY] : %s", fmt.Sprintf(format, args...))
}
