// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1_test

import (
	"log"

	"github.com/ultramcu/go-ict/sp1"
)

// ExampleDevice shows printing a small centred receipt. Write* calls are
// non-blocking; a single drainer goroutine paces the bytes to the SP1.
func ExampleDevice() {
	dev, err := sp1.New("/dev/ttyUSB1", 9600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer dev.Close()

	go dev.Run()

	dev.Init() // ESC/POS mode + reset
	dev.PrintfFormat(sp1.Large, sp1.Center, "RECEIPT")
	dev.LineFeed()
	dev.Printf("Total: %d", 100)
	dev.Execute() // commit the job (CR); drainer idles before the next one
}
