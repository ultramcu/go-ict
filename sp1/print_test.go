// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"bytes"
	"testing"
)

// newQueueDevice returns a port-less Device whose write queue can be
// inspected directly with drainQueued (no drainer goroutine, so no
// commit-gap timing involved).
func newQueueDevice() *Device {
	return &Device{
		queue: newByteQueue(queueCapacity),
		timer: newElapsedTimer(),
		done:  make(chan struct{}),
	}
}

// drainQueued pops every currently-buffered byte from the write queue.
func drainQueued(d *Device) []byte {
	var out []byte
	for {
		select {
		case b := <-d.queue.ch:
			out = append(out, b)
		default:
			return out
		}
	}
}

func TestPrintCommandBytes(t *testing.T) {
	cases := []struct {
		name string
		do   func(*Device)
		want []byte
	}{
		{"SetFontSizeSmall", func(d *Device) { d.SetFontSize(Small) }, []byte{ESC, 'M', '0'}},
		{"SetFontSizeNormal", func(d *Device) { d.SetFontSize(Normal) }, []byte{ESC, 'M', '1'}},
		{"SetFontSizeLarge", func(d *Device) { d.SetFontSize(Large) }, []byte{ESC, 'M', '2'}},
		{"SetUnderlineOn", func(d *Device) { d.SetUnderline(true) }, []byte{ESC, '-', '1'}},
		{"SetUnderlineOff", func(d *Device) { d.SetUnderline(false) }, []byte{ESC, '-', '0'}},
		{"ActivateESCPOS", func(d *Device) { d.ActivateESCPOSMode() }, []byte{ESC, '#'}},
		{"Reset", func(d *Device) { d.PrinterResetToInitial() }, []byte{ESC, '@'}},
		{"HighlightOn", func(d *Device) { d.CharactersHighlight(true) }, []byte{GS, 'B', '1'}},
		{"HighlightOff", func(d *Device) { d.CharactersHighlight(false) }, []byte{GS, 'B', '0'}},
		{"AutoClearOff", func(d *Device) { d.TurnOffCommunicationDataAutoClearMode() }, []byte{ESC, 'c', '5'}},
		{"Execute", func(d *Device) { d.Execute() }, []byte{CR}},
		{"LineFeed", func(d *Device) { d.LineFeed() }, []byte{'\n'}},
		{"Printf", func(d *Device) { d.Printf("Hi%d", 5) }, []byte("Hi5")},
		{"GetSysInfo", func(d *Device) { d.GetPrinterSystemInformation() }, []byte{ESC, 0x28, 'T'}},
		{"GenerateQRCode", func(d *Device) { d.GenerateQRCode("AB") }, []byte{GS, 'k', QRCode, 2, 0, 'A', 'B'}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := newQueueDevice()
			c.do(d)
			if got := drainQueued(d); !bytes.Equal(got, c.want) {
				t.Errorf("%s enqueued % X, want % X", c.name, got, c.want)
			}
		})
	}
}
