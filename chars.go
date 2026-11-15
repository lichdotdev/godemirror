package godemirror

//go:generate node internal/cmgen/extenders.mjs $CODEMIRROR_NODE_MODULES extenders_generated.go

import (
	"sort"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// findClusterBreak, the character categories and countColumn port
// @codemirror/state 6.7.6's char.ts, charcategory.ts and column.ts, and the
// @marijn/find-cluster-break 1.0.4 behind its findClusterBreak. CodeMirror
// walks UTF-16 code units, so the cluster walk runs on the UTF-16 form of
// the text and only its result turns back into bytes.

const zwj = 0x200d

// findClusterBreak is the next grapheme cluster break after pos in s
// (forward) or before it, both byte offsets, or pos when there is none.
// includeExtending takes combining marks into the cluster before them.
func findClusterBreak(s string, pos int, forward, includeExtending bool) int {
	units, at := utf16Of(s, pos)
	var found int
	if forward {
		found = nextClusterBreak(units, at, includeExtending)
	} else {
		found = prevClusterBreak(units, at, includeExtending)
	}
	return byteOffsetOf(s, found)
}

// utf16Of is s in UTF-16 code units, and pos, a byte offset into s, as an
// offset into them.
func utf16Of(s string, pos int) (units []uint16, at int) {
	at = -1
	for i, r := range s {
		if i == pos {
			at = len(units)
		}
		units = utf16.AppendRune(units, r)
	}
	if at < 0 {
		at = len(units)
	}
	return units, at
}

// byteOffsetOf is the byte offset into s of unit, an offset into its UTF-16
// code units that falls between two characters.
func byteOffsetOf(s string, unit int) int {
	n := 0
	for i, r := range s {
		if n >= unit {
			return i
		}
		n += utf16.RuneLen(r)
	}
	return len(s)
}

func nextClusterBreak(s []uint16, pos int, includeExtending bool) int {
	if pos == len(s) {
		return pos
	}
	if pos > 0 && surrogateLow(s[pos]) && surrogateHigh(s[pos-1]) {
		pos--
	}
	prev := codePointAt(s, pos)
	pos += codePointSize(prev)
	for pos < len(s) {
		next := codePointAt(s, pos)
		switch {
		case prev == zwj || next == zwj || includeExtending && isExtendingChar(next):
			pos += codePointSize(next)
			prev = next
		case isRegionalIndicator(next):
			countBefore := 0
			for i := pos - 2; i >= 0 && isRegionalIndicator(codePointAt(s, i)); i -= 2 {
				countBefore++
			}
			if countBefore%2 == 0 {
				return pos
			}
			pos += 2
		default:
			return pos
		}
	}
	return pos
}

func prevClusterBreak(s []uint16, pos int, includeExtending bool) int {
	for ; pos > 1; pos-- {
		if found := nextClusterBreak(s, pos-2, includeExtending); found < pos {
			return found
		}
	}
	return 0
}

func isExtendingChar(code rune) bool {
	if code < 768 {
		return false
	}
	i := sort.Search(len(extenders), func(i int) bool { return extenders[i][1] > code })
	return i < len(extenders) && extenders[i][0] <= code
}

func isRegionalIndicator(code rune) bool { return code >= 0x1f1e6 && code <= 0x1f1ff }

func codePointAt(s []uint16, pos int) rune {
	c0 := s[pos]
	if !surrogateHigh(c0) || pos+1 == len(s) {
		return rune(c0)
	}
	c1 := s[pos+1]
	if !surrogateLow(c1) {
		return rune(c0)
	}
	return utf16.DecodeRune(rune(c0), rune(c1))
}

func surrogateLow(c uint16) bool  { return c >= 0xdc00 && c < 0xe000 }
func surrogateHigh(c uint16) bool { return c >= 0xd800 && c < 0xdc00 }

func codePointSize(code rune) int {
	if code < 0x10000 {
		return 1
	}
	return 2
}

// charCategory is what CodeMirror's charCategorizer sorts a character into.
type charCategory int

const (
	categoryWord charCategory = iota
	categorySpace
	categoryOther
)

// categorize is CodeMirror's default charCategorizer, the one with no
// language "wordChars": a word holds letters, numbers or "_", a space only
// whitespace.
func categorize(char string) charCategory {
	switch {
	case allJSSpace(char):
		return categorySpace
	case hasWordChar(char):
		return categoryWord
	}
	return categoryOther
}

// allJSSpace reports whether every character of s is one JavaScript's \s
// matches, which is not the set unicode.IsSpace holds.
func allJSSpace(s string) bool {
	for _, r := range s {
		switch {
		case r == '\t', r == '\n', r == '\v', r == '\f', r == '\r', r == ' ', r == 0xa0, r == 0x1680,
			r >= 0x2000 && r <= 0x200a, r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000, r == 0xfeff:
		default:
			return false
		}
	}
	return true
}

// hasWordChar reports whether s holds a character of
// [\p{Alphabetic}\p{Number}_].
func hasWordChar(s string) bool {
	for _, r := range s {
		if r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) ||
			unicode.Is(unicode.Other_Alphabetic, r) {
			return true
		}
	}
	return false
}

// countColumn is the column at byte to of s, a tab reaching the next
// multiple of tabSize and a grapheme cluster taking one column.
func countColumn(s string, tabSize, to int) int {
	n := 0
	for i := 0; i < to && i < len(s); {
		if s[i] == '\t' {
			n += tabSize - n%tabSize
			i++
		} else {
			n++
			i = findClusterBreak(s, i, true, true)
		}
	}
	return n
}

// findColumn is the byte offset into s at column col, as countColumn counts
// them, and whether s reaches that column; when it does not, the offset is
// len(s).
func findColumn(s string, col, tabSize int) (offset int, reached bool) {
	for i, n := 0, 0; ; {
		if n >= col {
			return i, true
		}
		if i == len(s) {
			return len(s), false
		}
		if s[i] == '\t' {
			n += tabSize - n%tabSize
		} else {
			n++
		}
		i = findClusterBreak(s, i, true, true)
	}
}

// runeEnd is the offset just past the character that starts at pos in s.
func runeEnd(s string, pos int) int {
	_, n := utf8.DecodeRuneInString(s[pos:])
	return pos + n
}
