// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

// Package sp1 drives the ICT SP1 thermal receipt printer over a
// serial port. The printer speaks an ESC/POS-style command set with
// a few ICT-specific quirks (notably: a print job is committed only
// when a CR (0x0D) is sent, and the printer needs a small idle gap
// after each commit before it will accept the next batch of bytes).
//
// All Write/Print* calls go into an internal write queue; a single
// background goroutine drains the queue, inserts the required idle
// gap after every CR, and pushes bytes to the serial port. This
// keeps caller code synchronous and lets you mix small per-line
// writes without having to think about the protocol's pacing rules.
//
// Typical usage:
//
//	dev, err := sp1.New("/dev/ttyUSB1", 9600, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer dev.Close()
//	go dev.Run()
//
//	dev.Init()                                    // ESC # ; ESC @
//	dev.PrintfFormat(sp1.Large, sp1.Center, "Receipt\n")
//	dev.SetFontSize(sp1.Normal)
//	dev.Printf("Item .... 99.00 THB\n")
//	dev.LineFeed()
//	dev.Execute()                                 // CR -- commits
package sp1
