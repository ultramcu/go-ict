// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"bytes"
	"testing"
	"time"
)

// TestPrintfFormat verifies that PrintfFormat emits the correct ESC/POS
// font-select prefix followed by the alignment-padded text.
func TestPrintfFormat(t *testing.T) {
	cases := []struct {
		name   string
		size   FontSize
		align  Alignment
		text   string
		checkF func([]byte) bool // returns true if bytes look correct
	}{
		{
			name:  "Large/Center emits ESC M 2 then padded text",
			size:  Large,
			align: Center,
			text:  "RECEIPT",
			checkF: func(got []byte) bool {
				// First 3 bytes must be the SetFontSize(Large) command.
				if len(got) < 3 {
					return false
				}
				if got[0] != ESC || got[1] != 'M' || got[2] != '2' {
					return false
				}
				// Remainder should be the aligned text.
				rest := string(got[3:])
				aligned := AlignmentTextWithSpace(Large, Center, "RECEIPT")
				return rest == aligned
			},
		},
		{
			name:  "Normal/Left emits ESC M 1 then text unchanged",
			size:  Normal,
			align: Left,
			text:  "hello",
			checkF: func(got []byte) bool {
				if len(got) < 3+5 {
					return false
				}
				if got[0] != ESC || got[1] != 'M' || got[2] != '1' {
					return false
				}
				return string(got[3:]) == "hello"
			},
		},
		{
			name:  "Small/Right emits ESC M 0 then right-aligned text",
			size:  Small,
			align: Right,
			text:  "42",
			checkF: func(got []byte) bool {
				if len(got) < 3 {
					return false
				}
				if got[0] != ESC || got[1] != 'M' || got[2] != '0' {
					return false
				}
				rest := string(got[3:])
				aligned := AlignmentTextWithSpace(Small, Right, "42")
				return rest == aligned
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newQueueDevice()
			d.PrintfFormat(tc.size, tc.align, "%s", tc.text)
			got := drainQueued(d)
			if !tc.checkF(got) {
				t.Errorf("PrintfFormat(%v, %v, %q) enqueued % X — check failed",
					tc.size, tc.align, tc.text, got)
			}
		})
	}
}

// TestSetQRCodeSize verifies the GS ( k command bytes and the size clamping
// logic (clamp to [1, 7]).
//
// SetQRCodeSize calls time.Sleep(1s) after writing — we run it in a goroutine,
// grab the bytes while it sleeps, then close the device and wait for it to
// return. Sub-tests run in parallel so the total wall time is ~1 s not 5 s.
func TestSetQRCodeSize(t *testing.T) {
	cases := []struct {
		name  string
		input int
		want  byte // clamped size byte, last byte of the 8-byte command
	}{
		{"clamp below 1", 0, 1},
		{"min 1", 1, 1},
		{"mid 4", 4, 4},
		{"max 7", 7, 7},
		{"clamp above 7", 99, 7},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := newQueueDevice()

			callerDone := make(chan struct{})
			go func() {
				d.SetQRCodeSize(tc.input)
				close(callerDone)
			}()

			// Bytes land in the queue before the 1-s firmware sleep fires.
			// Poll briefly then close to unblock the goroutine.
			var got []byte
			deadline := time.Now().Add(200 * time.Millisecond)
			for time.Now().Before(deadline) {
				got = drainQueued(d)
				if len(got) >= 8 {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			d.Close()
			<-callerDone

			wantSeq := []byte{GS, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, tc.want}
			if !bytes.Equal(got, wantSeq) {
				t.Errorf("SetQRCodeSize(%d) enqueued % X, want % X",
					tc.input, got, wantSeq)
			}
		})
	}
}

// TestGenerateQRCode_LargeData verifies the 65535-byte truncation guard.
// If data exceeds 0xFFFF bytes, GenerateQRCode must truncate and the length
// bytes in the 5-byte header must encode 0xFFFF.
//
// We use a very large queue (≥ 5 + 0xFFFF bytes) so the write does not block,
// then inspect only the first 5 header bytes.
func TestGenerateQRCode_LargeData(t *testing.T) {
	// Device with a queue large enough to hold the full payload without a drainer.
	d := &Device{
		queue: newByteQueue(0x10010), // 65552 — more than 5 + 0xFFFF
		timer: newElapsedTimer(),
		done:  make(chan struct{}),
	}

	// Build a string that is one byte over the limit.
	big := make([]byte, 0x10000) // 65536 bytes
	for i := range big {
		big[i] = 'X'
	}
	d.GenerateQRCode(string(big))

	// Pop just the first 5 header bytes.
	var header []byte
	for i := 0; i < 5; i++ {
		b, ok := d.queue.Dequeue()
		if !ok {
			t.Fatalf("queue closed before header byte %d", i)
		}
		header = append(header, b)
	}
	d.Close()

	// GS 'k' QRCode lenLo lenHi
	if header[0] != GS || header[1] != 'k' || header[2] != QRCode {
		t.Errorf("unexpected header prefix: % X", header[:3])
	}
	lenLo, lenHi := header[3], header[4]
	encoded := int(lenLo) + int(lenHi)*256
	if encoded != 0xFFFF {
		t.Errorf("encoded length = %d, want 65535", encoded)
	}
}

// TestGenerateQRCode_MultiByteLength verifies the two-byte length encoding for
// payloads that exceed 255 bytes (lenHi > 0).
func TestGenerateQRCode_MultiByteLength(t *testing.T) {
	// 300 bytes → lenLo = 44, lenHi = 1 (300 = 1*256 + 44)
	payload := make([]byte, 300)
	for i := range payload {
		payload[i] = 'Q'
	}

	d := newQueueDevice()
	d.GenerateQRCode(string(payload))
	got := drainQueued(d)

	if len(got) < 5 {
		t.Fatalf("only %d bytes, want at least 5", len(got))
	}
	lenLo, lenHi := got[3], got[4]
	if lenLo != 44 || lenHi != 1 {
		t.Errorf("length bytes = (%d, %d), want (44, 1)", lenLo, lenHi)
	}
}

// TestInitBytes verifies Init emits ActivateESCPOSMode then PrinterResetToInitial.
func TestInitBytes(t *testing.T) {
	d := newQueueDevice()
	d.Init()
	got := drainQueued(d)
	want := []byte{ESC, '#', ESC, '@'}
	if !bytes.Equal(got, want) {
		t.Errorf("Init enqueued % X, want % X", got, want)
	}
}
