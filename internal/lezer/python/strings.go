// Ported from @lezer/python 1.1.19 (src/tokens.js), MIT licensed, Copyright
// (C) 2018 by Marijn Haverbeke and others. See ../LICENSE.

package python

import "github.com/lichdotdev/godemirror/internal/lezer"

// Digits each escape kind reads at most.
const (
	octalEscapeDigits  = 2
	hexEscapeDigits    = 2
	shortUnicodeDigits = 4
	longUnicodeDigits  = 8
)

// stringKind is how the string the context is in reads its content.
type stringKind struct {
	quote                 int
	long, escapes, format bool
}

func stringKindOf(flags int) stringKind {
	quote := int(charSingleQuote)
	if flags&flagDoubleQuote != 0 {
		quote = charDoubleQuote
	}
	return stringKind{quote: quote, long: flags&flagLong != 0, escapes: flags&flagRaw == 0, format: flags&flagFormat != 0}
}

// closesAt reports whether the stream is at the string's closing quote.
func (k stringKind) closesAt(in *lezer.InputStream) bool {
	return in.Next == k.quote && (!k.long || in.Peek(1) == k.quote && in.Peek(2) == k.quote)
}

// stringTokens reads a string's content up to the next escape, replacement
// field, closing quote or (for long strings, past) line break, or, at one
// of those, that token.
var stringTokens = lezer.Tokenizer{Token: func(in *lezer.InputStream, s *lezer.Stack) {
	k := stringKindOf(contextOf(s).flags)
	start := in.Pos
scan:
	for in.Next >= 0 {
		switch {
		case k.format && in.Next == charBraceOpen:
			if in.Peek(1) == charBraceOpen {
				in.Advance(2)
				continue
			}
			if in.Pos == start {
				in.AcceptToken(termReplacementStart, 1)
				return
			}
			break scan
		case k.escapes && in.Next == charBackslash:
			if in.Pos == start {
				readEscape(in)
				return
			}
			break scan
		case in.Next == charBackslash && !k.escapes && in.Peek(1) > -1:
			// Raw strings still ignore escaped quotes, weirdly.
			in.Advance(2)
		case k.closesAt(in):
			if in.Pos == start {
				in.AcceptToken(termStringEnd, closingLength(k))
				return
			}
			break scan
		case in.Next == charNewline:
			if k.long {
				in.Advance(1)
			} else if in.Pos == start {
				in.AcceptToken(termStringEnd, 0)
				return
			}
			break scan
		default:
			in.Advance(1)
		}
	}
	if in.Pos > start {
		in.AcceptToken(termStringContent, 0)
	}
}}

func closingLength(k stringKind) int {
	if k.long {
		return 3
	}
	return 1
}

func readEscape(in *lezer.InputStream) {
	in.Advance(1)
	if escaped := in.Next; escaped >= 0 {
		in.Advance(1)
		skipEscape(in, escaped)
	}
	in.AcceptToken(nodeEscape, 0)
}

func skipEscape(in *lezer.InputStream, ch int) {
	switch ch {
	case 'o':
		skipDigits(in, octalEscapeDigits, func(ch int) bool { return ch >= '0' && ch <= '7' })
	case 'x':
		skipDigits(in, hexEscapeDigits, isHex)
	case 'u':
		skipDigits(in, shortUnicodeDigits, isHex)
	case 'U':
		skipDigits(in, longUnicodeDigits, isHex)
	case 'N':
		skipCharacterName(in)
	}
}

func skipDigits(in *lezer.InputStream, most int, isDigit func(int) bool) {
	for i := 0; i < most && isDigit(in.Next); i++ {
		in.Advance(1)
	}
}

// skipCharacterName skips a \N escape's {name}, stopping short at a quote
// or line break.
func skipCharacterName(in *lezer.InputStream) {
	if in.Next != charBraceOpen {
		return
	}
	in.Advance(1)
	for in.Next >= 0 && in.Next != charBraceClose && in.Next != charSingleQuote &&
		in.Next != charDoubleQuote && in.Next != charNewline {
		in.Advance(1)
	}
	if in.Next == charBraceClose {
		in.Advance(1)
	}
}
