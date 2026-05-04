// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"strings"
	"testing"
	"time"
)

func TestAddSpaceToFrontOfText(t *testing.T) {
	cases := []struct {
		text  string
		count int
		want  string
	}{
		{"hello", 0, "hello"},
		{"hello", -3, "hello"},
		{"hello", 3, "   hello"},
	}
	for _, tc := range cases {
		if got := AddSpaceToFrontOfText(tc.text, tc.count); got != tc.want {
			t.Errorf("AddSpaceToFrontOfText(%q, %d) = %q, want %q",
				tc.text, tc.count, got, tc.want)
		}
	}
}

func TestAlignmentTextWithSpace(t *testing.T) {
	cases := []struct {
		name  string
		size  FontSize
		align Alignment
		text  string
		want  string
	}{
		{"Large/Center pads half", Large, Center, "OK", strings.Repeat(" ", (MaxLargeChars-2)/2) + "OK"},
		{"Large/Right pads full", Large, Right, "OK", strings.Repeat(" ", MaxLargeChars-2) + "OK"},
		{"Normal/Center pads half", Normal, Center, "OK", strings.Repeat(" ", (MaxNormalChars-2)/2) + "OK"},
		{"Small/Right pads full", Small, Right, "AB", strings.Repeat(" ", MaxSmallChars-2) + "AB"},
		{"Left returns unchanged", Normal, Left, "hello", "hello"},
		{"Text wider than line returns unchanged", Large, Center, strings.Repeat("X", 32), strings.Repeat("X", 32)},
		{"UTF-8 counts runes not bytes", Normal, Center, "ห", strings.Repeat(" ", (MaxNormalChars-1)/2) + "ห"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AlignmentTextWithSpace(tc.size, tc.align, tc.text); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRemoveNewline(t *testing.T) {
	cases := map[string]string{
		"hello\n":   "hello",
		"hello":     "hello",
		"\n":        "",
		"hello\n\n": "hello\n",
		"":          "",
	}
	for in, want := range cases {
		if got := RemoveNewline(in); got != want {
			t.Errorf("RemoveNewline(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFontSizeString(t *testing.T) {
	cases := map[FontSize]string{
		Small:        "Small",
		Normal:       "Normal",
		Large:        "Large",
		FontSize(99): "Unknown",
	}
	for f, want := range cases {
		if got := f.String(); got != want {
			t.Errorf("(%d).String() = %q, want %q", int(f), got, want)
		}
	}
}

func TestAlignmentString(t *testing.T) {
	cases := map[Alignment]string{
		Left:          "Left",
		Right:         "Right",
		Center:        "Center",
		Alignment(99): "Unknown",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("(%d).String() = %q, want %q", int(a), got, want)
		}
	}
}

func TestByteQueue_FIFO(t *testing.T) {
	q := newByteQueue(4)
	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)

	for i, want := range []byte{1, 2, 3} {
		got, ok := q.Dequeue()
		if !ok {
			t.Fatalf("Dequeue %d: !ok", i)
		}
		if got != want {
			t.Errorf("Dequeue %d = %d, want %d", i, got, want)
		}
	}
}

func TestByteQueue_CloseSignals(t *testing.T) {
	q := newByteQueue(2)
	q.Enqueue(7)
	q.Close()

	// Drain the remaining buffered byte.
	got, ok := q.Dequeue()
	if !ok || got != 7 {
		t.Errorf("first Dequeue after Close = (%d, %v), want (7, true)", got, ok)
	}
	// Second Dequeue should return the closed/zero value.
	_, ok = q.Dequeue()
	if ok {
		t.Errorf("second Dequeue after Close: ok = true, want false")
	}
}

func TestElapsedTimer_AdvancesOnEachCall(t *testing.T) {
	tm := newElapsedTimer()
	first := tm.SinceLastCall()
	if first < 0 {
		t.Errorf("first SinceLastCall = %v, want >= 0", first)
	}

	time.Sleep(15 * time.Millisecond)

	second := tm.SinceLastCall()
	// second should be at least the sleep we just did.
	if second < 10*time.Millisecond {
		t.Errorf("second SinceLastCall = %v, want >= 10ms", second)
	}
}
