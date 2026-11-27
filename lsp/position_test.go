package lsp

import "testing"

func TestColumnsConvertBetweenEncodings(t *testing.T) {
	// "a😀é=" is a (1 byte, 1 unit), 😀 (4 bytes, 2 UTF-16 units), é (2
	// bytes, 1 unit), =.
	line := "a😀é="
	for _, tc := range []struct {
		enc       PositionEncoding
		character int
		byteCol   int
	}{
		{UTF16, 0, 0}, {UTF16, 1, 1}, {UTF16, 3, 5}, {UTF16, 4, 7}, {UTF16, 5, 8},
		{UTF32, 2, 5}, {UTF32, 3, 7},
		{UTF8, 5, 5},
	} {
		if got := tc.enc.ByteColumn(line, tc.character); got != tc.byteCol {
			t.Errorf("%s ByteColumn(%d) = %d, want %d", tc.enc, tc.character, got, tc.byteCol)
		}
		if got := tc.enc.Character(line, tc.byteCol); got != tc.character {
			t.Errorf("%s Character(%d) = %d, want %d", tc.enc, tc.byteCol, got, tc.character)
		}
	}
	if got := UTF16.ByteColumn(line, 2); got != 1 {
		t.Errorf("a character inside a surrogate pair went to %d, want its start, 1", got)
	}
	if got := UTF16.ByteColumn(line, 99); got != len(line) {
		t.Errorf("a character past the end went to %d, want %d", got, len(line))
	}
}
