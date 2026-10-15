// Ported from @codemirror/streamparser 6.0.0 (StringStream), MIT licensed,
// Copyright (C) 2018-2021 by Marijn Haverbeke and others. See LICENSE.

package stream

// EOL is what Peek and Next return at the end of the line, where
// JavaScript's StringStream returns undefined.
const EOL = -1

// Stream is one line of a document, read by a mode one token at a time.
// Like JavaScript's StringStream it counts in UTF-16 code units, so a
// character outside the BMP is two units and a mode sees them one by one.
type Stream struct {
	line       []uint16
	start, pos int
}

// EOL reports whether the stream is at the end of the line.
func (s *Stream) EOL() bool { return s.pos >= len(s.line) }

// SOL reports whether the stream is at the start of the line.
func (s *Stream) SOL() bool { return s.pos == 0 }

// Line is the whole line.
func (s *Stream) Line() []uint16 { return s.line }

// Current is the text of the token read so far.
func (s *Stream) Current() []uint16 { return s.line[s.start:s.pos] }

// Peek is the next code unit, EOL at the end of the line.
func (s *Stream) Peek() int {
	if s.EOL() {
		return EOL
	}
	return int(s.line[s.pos])
}

// Next reads the next code unit, EOL (and no move) at the end of the line.
func (s *Stream) Next() int {
	if s.EOL() {
		return EOL
	}
	s.pos++
	return int(s.line[s.pos-1])
}

// Eat reads the next code unit if it is c.
func (s *Stream) Eat(c uint16) bool {
	return s.EatFunc(func(u uint16) bool { return u == c })
}

// EatFunc reads the next code unit if match accepts it.
func (s *Stream) EatFunc(match func(uint16) bool) bool {
	if s.EOL() || !match(s.line[s.pos]) {
		return false
	}
	s.pos++
	return true
}

// EatWhile reads code units as long as match accepts them, and reports
// whether it read any.
func (s *Stream) EatWhile(match func(uint16) bool) bool {
	start := s.pos
	for s.EatFunc(match) {
	}
	return s.pos > start
}

// EatSpace reads whitespace, and reports whether it read any.
func (s *Stream) EatSpace() bool { return s.EatWhile(IsSpace) }

// SkipToEnd moves to the end of the line.
func (s *Stream) SkipToEnd() { s.pos = len(s.line) }

// SkipTo moves to just before the next c on the line, and reports whether
// there was one; without one the stream stays where it is.
func (s *Stream) SkipTo(c uint16) bool {
	for i := s.pos; i < len(s.line); i++ {
		if s.line[i] == c {
			s.pos = i
			return true
		}
	}
	return false
}

// BackUp moves back n code units.
func (s *Stream) BackUp(n int) { s.pos -= n }

// Match reads text if the line continues with it, as StringStream's match
// does with a string.
func (s *Stream) Match(text string) bool {
	if !s.LookingAt(text) {
		return false
	}
	s.pos += len(text)
	return true
}

// LookingAt reports whether the line continues with text, an ASCII string,
// without reading it: match(text, false).
func (s *Stream) LookingAt(text string) bool {
	rest := s.line[s.pos:]
	if len(rest) < len(text) {
		return false
	}
	for i := range len(text) {
		if rest[i] != uint16(text[i]) {
			return false
		}
	}
	return true
}

// MatchFunc reads what match finds at the start of the rest of the line,
// the job of a regular expression starting with ^ in StringStream's
// match: match returns the length it matched, or -1 for none.
func (s *Stream) MatchFunc(match func(rest []uint16) int) bool {
	n := match(s.line[s.pos:])
	if n < 0 {
		return false
	}
	s.pos += n
	return true
}

// IsWord is JavaScript's \w: an ASCII letter, digit or underscore.
func IsWord(u uint16) bool {
	return IsDigit(u) || u == '_' || 'a' <= u && u <= 'z' || 'A' <= u && u <= 'Z'
}

// IsDigit is JavaScript's \d: an ASCII digit.
func IsDigit(u uint16) bool { return '0' <= u && u <= '9' }

// IsSpace is JavaScript's \s, which StringStream's eatSpace extends with
// U+00A0, already part of it.
func IsSpace(u uint16) bool {
	switch u {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x00a0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return 0x2000 <= u && u <= 0x200a
}
