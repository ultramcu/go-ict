// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"strings"
	"unicode/utf8"
)

// AlignmentTextWithSpace returns text padded with leading spaces so
// that, when rendered in the given font size, it appears centered
// or right-aligned on the receipt. A Left alignment (the default) is
// returned unchanged. Strings already wider than the font's line
// budget are returned unchanged.
//
// The width is measured in Unicode code points (runes), not bytes,
// so multi-byte UTF-8 characters each occupy one column slot.
func AlignmentTextWithSpace(fontSize FontSize, align Alignment, text string) string {
	if align == Left {
		return text
	}

	var maxChars int
	switch fontSize {
	case Large:
		maxChars = MaxLargeChars
	case Normal:
		maxChars = MaxNormalChars
	case Small:
		maxChars = MaxSmallChars
	default:
		return text
	}

	textLen := utf8.RuneCountInString(text)
	if textLen >= maxChars {
		return text
	}

	pad := maxChars - textLen
	if align == Center {
		pad /= 2
	}
	return AddSpaceToFrontOfText(text, pad)
}

// AddSpaceToFrontOfText prepends `count` spaces to `text`. A count of
// zero or less returns text unchanged.
func AddSpaceToFrontOfText(text string, count int) string {
	if count <= 0 {
		return text
	}
	return strings.Repeat(" ", count) + text
}

// RemoveNewline returns s with a single trailing newline stripped.
// Used by the log helpers to keep one-line log entries on one line.
func RemoveNewline(s string) string {
	return strings.TrimSuffix(s, "\n")
}
