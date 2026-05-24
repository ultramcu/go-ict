// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package l77

import "testing"

// FuzzFeedByte asserts the inbound state machine never panics on
// arbitrary byte streams -- the "device sends garbage on a noisy line"
// case. Run: go test -run=- -fuzz=FuzzFeedByte
func FuzzFeedByte(f *testing.F) {
	f.Add([]byte{0x80, 0x8F})       // power-up handshake
	f.Add([]byte{0x81, 0x09, 0x10}) // escrow B100 then stack
	f.Add([]byte{0x4C})             // version header alone (truncated)
	f.Add([]byte{0x5B})             // info header alone (truncated)

	f.Fuzz(func(t *testing.T, data []byte) {
		d := newTestDevice()
		for _, b := range data {
			d.FeedByte(b) // invariant: never panics
		}
	})
}
