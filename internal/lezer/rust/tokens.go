// Ported from @lezer/rust 1.0.3 (src/tokens.js), MIT licensed, Copyright (C)
// 2018 by Marijn Haverbeke and others. See ../LICENSE.

package rust

import "github.com/lichdotdev/godemirror/internal/lezer"

// Terms of @lezer/rust 1.0.3's grammar, from its generated parser.terms.js
// (inlined at the top of dist/index.js).
const (
	termClosureParamDelim = 1
	termTpOpen            = 2
	termTpClose           = 3
	termRawString         = 4
	termFloat             = 5
	termAndAnd            = 301
)

const lastASCII = 0x7f

// literalTokens reads float literals and raw (byte) strings; integers are
// left to the grammar's own tokens.
var literalTokens = lezer.Tokenizer{Token: func(in *lezer.InputStream, _ *lezer.Stack) {
	if isDigit(in.Next) {
		readFloat(in)
	} else if in.Next == 'b' || in.Next == 'r' {
		readRawString(in)
	}
}}

func readFloat(in *lezer.InputStream) {
	isFloat := false
	advanceDigits(in)
	if in.Next == '.' {
		isFloat = true
		in.Advance(1)
		if isDigit(in.Next) {
			advanceDigits(in)
		} else if in.Next == '.' || in.Next > lastASCII || isWordChar(in.Next) {
			return
		}
	}
	if in.Next == 'e' || in.Next == 'E' {
		isFloat = true
		in.Advance(1)
		if in.Next == '+' || in.Next == '-' {
			in.Advance(1)
		}
		if !isDigitOrUnderscore(in.Next) {
			return
		}
		advanceDigits(in)
	}
	if in.Next == 'f' {
		if !isFloatSuffix(in) {
			return
		}
		in.Advance(len("f32"))
		isFloat = true
	}
	if isFloat {
		in.AcceptToken(termFloat, 0)
	}
}

// advanceDigits moves past the current character and every digit or
// underscore after it.
func advanceDigits(in *lezer.InputStream) {
	for {
		in.Advance(1)
		if !isDigitOrUnderscore(in.Next) {
			return
		}
	}
}

// isFloatSuffix reports whether the 'f' at the stream starts f32 or f64.
func isFloatSuffix(in *lezer.InputStream) bool {
	after := in.Peek(1)
	return after == '3' && in.Peek(2) == '2' || after == '6' && in.Peek(2) == '4'
}

func readRawString(in *lezer.InputStream) {
	if in.Next == 'b' {
		in.Advance(1)
	}
	if in.Next != 'r' {
		return
	}
	in.Advance(1)
	hashes := 0
	for in.Next == '#' {
		hashes++
		in.Advance(1)
	}
	if in.Next != '"' {
		return
	}
	in.Advance(1)
	for in.Next >= 0 {
		isQuote := in.Next == '"'
		in.Advance(1)
		if isQuote && closesRawString(in, hashes) {
			in.AcceptToken(termRawString, 0)
			return
		}
	}
}

// closesRawString consumes the hashes after a quote, reporting whether
// there were enough of them to end the string.
func closesRawString(in *lezer.InputStream, hashes int) bool {
	for range hashes {
		if in.Next != '#' {
			return false
		}
		in.Advance(1)
	}
	return true
}

var closureParam = lezer.Tokenizer{Token: func(in *lezer.InputStream, _ *lezer.Stack) {
	if in.Next == '|' {
		in.AcceptToken(termClosureParamDelim, 1)
	}
}}

var tpDelim = lezer.Tokenizer{Token: func(in *lezer.InputStream, _ *lezer.Stack) {
	if in.Next == '<' && in.Peek(1) != '=' {
		in.AcceptToken(termTpOpen, 1)
	} else if in.Next == '>' {
		in.AcceptToken(termTpClose, 1)
	}
}}

var logicAnd = lezer.Tokenizer{Contextual: true, Token: func(in *lezer.InputStream, s *lezer.Stack) {
	if in.Next == '&' && in.Peek(1) == '&' && s.CanShift(termAndAnd) {
		in.AcceptToken(termAndAnd, 2)
	}
}}

func isDigit(ch int) bool { return ch >= '0' && ch <= '9' }

func isDigitOrUnderscore(ch int) bool { return isDigit(ch) || ch == '_' }

// isWordChar is JavaScript's /\w/: ASCII letters, digits and underscore.
func isWordChar(ch int) bool {
	return isDigitOrUnderscore(ch) || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
}
