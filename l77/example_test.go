// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77_test

import (
	"log"

	"github.com/ultramcu/go-ict/l77"
)

// ExampleDevice shows the typical lifecycle: open the acceptor, wire up
// callbacks, run the read loop in its own goroutine, and close on exit.
func ExampleDevice() {
	dev, err := l77.New(
		"/dev/ttyUSB0",
		func(d *l77.Device, bill l77.BillType) { // bill entered escrow
			log.Printf("escrow: %d", bill.Value())
		},
		func(d *l77.Device, bill l77.BillType) { // bill stacked
			log.Printf("stacked: %d", bill.Value())
		},
		func(d *l77.Device, bill l77.BillType) { // bill rejected
			log.Printf("rejected: %d", bill.Value())
		},
		func(d *l77.Device, s l77.Status) { // status / error frames
			log.Printf("status: %s", s)
		},
		l77.AutoAccept, // automatically stack accepted bills
		nil,            // no logging
	)
	if err != nil {
		log.Fatal(err)
	}
	defer dev.Close()

	go dev.Run()

	// Parsed device info can be read concurrently via the accessors:
	_ = dev.GetSerialNumber()
}
