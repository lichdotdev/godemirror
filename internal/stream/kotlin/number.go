package kotlin

import "github.com/lichdotdev/godemirror/internal/stream"

// number is the length of the number rest starts with, -1 for none:
// kotlin's /^(?:0x[a-f\d_]+|0b[01_]+|(?:[\d_]+(\.\d+)?|\.\d+)(?:e[-+]?[\d_]+)?)(ul?|l|f)?/i.
// Every quantifier after the mantissa is optional, so the regexp's first
// match is always its longest one and nothing here backtracks.
func number(rest []uint16) int {
	n := radixNumber(rest, 'x', isHexOrUnderscore)
	if n < 0 {
		n = radixNumber(rest, 'b', isBinaryOrUnderscore)
	}
	if n < 0 {
		n = decimalNumber(rest)
	}
	if n < 0 {
		return -1
	}
	return n + suffix(rest[n:])
}

// radixNumber is the length of "0" and the letter (either case) followed by
// at least one unit digit accepts, -1 for none.
func radixNumber(rest []uint16, letter uint16, digit func(uint16) bool) int {
	if len(rest) < 2 || rest[0] != '0' || lower(rest[1]) != letter {
		return -1
	}
	run := runOf(rest[2:], digit)
	if run == 0 {
		return -1
	}
	return 2 + run
}

// decimalNumber is (?:[\d_]+(\.\d+)?|\.\d+)(?:e[-+]?[\d_]+)?.
func decimalNumber(rest []uint16) int {
	n := runOf(rest, isDigitOrUnderscore)
	switch {
	case n > 0:
		if n < len(rest) && rest[n] == '.' {
			if digits := runOf(rest[n+1:], stream.IsDigit); digits > 0 {
				n += 1 + digits
			}
		}
	case len(rest) > 0 && rest[0] == '.':
		digits := runOf(rest[1:], stream.IsDigit)
		if digits == 0 {
			return -1
		}
		n = 1 + digits
	default:
		return -1
	}
	return n + exponent(rest[n:])
}

// exponent is the length of e[-+]?[\d_]+ at the start of rest, 0 for none.
func exponent(rest []uint16) int {
	if len(rest) == 0 || lower(rest[0]) != 'e' {
		return 0
	}
	n := 1
	if n < len(rest) && (rest[n] == '-' || rest[n] == '+') {
		n++
	}
	digits := runOf(rest[n:], isDigitOrUnderscore)
	if digits == 0 {
		return 0
	}
	return n + digits
}

// suffix is the length of (ul?|l|f) at the start of rest, 0 for none.
func suffix(rest []uint16) int {
	if len(rest) == 0 {
		return 0
	}
	switch lower(rest[0]) {
	case 'u':
		if len(rest) > 1 && lower(rest[1]) == 'l' {
			return 2
		}
		return 1
	case 'l', 'f':
		return 1
	}
	return 0
}

// runOf is how many units at the start of rest accept accepts.
func runOf(rest []uint16, accept func(uint16) bool) int {
	n := 0
	for n < len(rest) && accept(rest[n]) {
		n++
	}
	return n
}

func isDigitOrUnderscore(u uint16) bool { return stream.IsDigit(u) || u == '_' }

func isBinaryOrUnderscore(u uint16) bool { return u == '0' || u == '1' || u == '_' }

func isHexOrUnderscore(u uint16) bool {
	return isDigitOrUnderscore(u) || 'a' <= lower(u) && lower(u) <= 'f'
}

// lower folds an ASCII letter to lower case, as the regexp's i flag does
// for the letters it names.
func lower(u uint16) uint16 {
	if 'A' <= u && u <= 'Z' {
		return u + 'a' - 'A'
	}
	return u
}
