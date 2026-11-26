package lsp

import "unicode/utf16"

// PositionEncoding is the unit a Position's Character counts in, as client
// and server agree on it at initialize.
type PositionEncoding string

// The position encodings of LSP 3.17. UTF-16 is the protocol's default,
// the one a server that names none speaks.
const (
	UTF8  PositionEncoding = "utf-8"
	UTF16 PositionEncoding = "utf-16"
	UTF32 PositionEncoding = "utf-32"
)

// ByteColumn is the byte offset in line (one line's text, without its line
// break) of character, counted in enc. A character past the end of line,
// as LSP allows, is its end; one inside a UTF-8 sequence or a surrogate
// pair goes to the start of that character.
func (enc PositionEncoding) ByteColumn(line string, character int) int {
	if enc == UTF8 {
		return min(max(character, 0), len(line))
	}
	n := 0
	for at, r := range line {
		n += enc.width(r)
		if n > character {
			return at
		}
	}
	return len(line)
}

// Character is the column of byteCol, a byte offset in line, counted in
// enc.
func (enc PositionEncoding) Character(line string, byteCol int) int {
	byteCol = min(max(byteCol, 0), len(line))
	if enc == UTF8 {
		return byteCol
	}
	n := 0
	for at, r := range line {
		if at >= byteCol {
			break
		}
		n += enc.width(r)
	}
	return n
}

// width is how many units of enc, UTF-16 or UTF-32, r takes.
func (enc PositionEncoding) width(r rune) int {
	if enc == UTF32 {
		return 1
	}
	return max(utf16.RuneLen(r), 1)
}
