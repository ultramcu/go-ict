// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import "fmt"

// Logf matches log.Printf's signature. Pass nil when constructing a
// Device to disable logging.
type Logf func(format string, args ...interface{}) string

// SetLogFunc installs / replaces the device's log function. Pass nil
// to disable logging.
func (d *Device) SetLogFunc(f Logf) {
	d.logf = f
}

// log writes a generic line tagged "[   ]".
func (d *Device) log(format string, args ...interface{}) {
	if d.logf == nil {
		return
	}
	d.logf("[   ] : %s", fmt.Sprintf(format, args...))
}

// logSP1 writes a line tagged "[SP1]" -- bytes received from the printer.
func (d *Device) logSP1(format string, args ...interface{}) {
	if d.logf == nil {
		return
	}
	d.logf("[SP1] : %s", fmt.Sprintf(format, args...))
}

// logCPU writes a line tagged "[CPU]" -- bytes the controller sent to
// the printer.
func (d *Device) logCPU(format string, args ...interface{}) {
	if d.logf == nil {
		return
	}
	d.logf("[CPU] : %s", fmt.Sprintf(format, args...))
}
