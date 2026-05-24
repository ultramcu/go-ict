// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

// ESC/POS control bytes used throughout the package.
const (
	ESC byte = 0x1B
	FS  byte = 0x1C
	GS  byte = 0x1D
	DEL byte = 0x10
	CR  byte = 0x0D // commits a print job on the SP1
	LF  byte = 0x0A
)

// Barcode symbology codes (passed to GS k commands).
const (
	UPC_A   byte = 65
	UPC_E   byte = 66
	EAN_8   byte = 67
	EAN_13  byte = 68
	CODE39  byte = 69
	Code128 byte = 70
	ITF_25  byte = 71
	QRCode  byte = 72
	QRCode2 byte = 90
)

// Alignment is the horizontal alignment of a printed line. The
// SP1's native alignment command is FS A {0,1,2}; this package also
// emulates alignment in software via leading spaces (see
// AlignmentTextWithSpace) which works on every font size.
type Alignment int

const (
	Left   Alignment = 0
	Right  Alignment = 1
	Center Alignment = 2
)

// String renders an Alignment as a short label for logging.
func (a Alignment) String() string {
	switch a {
	case Left:
		return "Left"
	case Right:
		return "Right"
	case Center:
		return "Center"
	default:
		return "Unknown"
	}
}

// FontSize is the SP1's three font sizes, set with ESC M {0,1,2}.
//
//	Small   ESC M 0  -- 8x13 glyphs, ~48 chars per line
//	Normal  ESC M 1  -- 16x24 glyphs, ~24 chars per line
//	Large   ESC M 2  -- 24x36 glyphs, ~16 chars per line
type FontSize int

const (
	Small  FontSize = 0
	Normal FontSize = 1
	Large  FontSize = 2
)

// String renders a FontSize as a short label for logging.
func (f FontSize) String() string {
	switch f {
	case Small:
		return "Small"
	case Normal:
		return "Normal"
	case Large:
		return "Large"
	default:
		return "Unknown"
	}
}

// Per-font line-width budgets used by AlignmentTextWithSpace. They
// are byte-rune counts (one Unicode rune per character), not byte
// counts, so multi-byte UTF-8 characters take one slot just like
// ASCII does.
const (
	MaxLargeChars  = 16
	MaxNormalChars = 24
	MaxSmallChars  = 48
)
