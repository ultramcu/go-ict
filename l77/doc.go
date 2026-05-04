// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

// Package l77 drives the ICT L77 bill acceptor over a serial port.
//
// The L77 speaks a simple single-byte command/response protocol. It
// pushes events to the controller (this package) — bill arriving in
// escrow, bill stacked, status changes, power-up handshake — and the
// controller answers with single-byte requests (accept, reject,
// hold, enable, disable, reset, etc.).
//
// Typical usage:
//
//	dev, err := l77.New(
//	    "/dev/ttyUSB0",
//	    onEscrow,        // bill arrived in escrow
//	    onStack,         // bill went into the stacker
//	    onReject,        // bill was rejected
//	    onStatus,        // device status / error
//	    l77.AutoHold,    // hold the bill in escrow until ReqBillAccept/Reject
//	    nil,             // optional logf
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer dev.Close()
//	go dev.Run()
//
//	// later, in onEscrow:
//	dev.ReqBillAccept()
//
// The Device's read loop runs in the goroutine that calls Run() and
// stops when Close() is invoked. All public methods are safe for
// concurrent use; serial writes are mutex-protected internally.
package l77
