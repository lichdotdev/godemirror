// Ported from @lezer/javascript 1.5.6 (src/tokens.js), MIT licensed,
// Copyright (C) 2018 by Marijn Haverbeke and others. See ../LICENSE.

package javascript

import (
	"slices"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

var space = []int{9, 10, 11, 12, 13, 32, 133, 160, 5760, 8192, 8193, 8194, 8195, 8196, 8197, 8198, 8199, 8200,
	8201, 8202, 8232, 8233, 8239, 8287, 12288}

const (
	braceR    = '}'
	semicolon = ';'
	slash     = '/'
	star      = '*'
	plus      = '+'
	minus     = '-'
	lt        = '<'
	comma     = ','
	question  = '?'
	dot       = '.'
	bracketL  = '['
)

var externals = lezer.Externals{
	Tokenizers: map[string]lezer.Tokenizer{
		"insertSemicolon": {Token: insertSemicolon, Contextual: true, Fallback: true},
		"noSemicolon":     {Token: noSemicolon, Contextual: true},
		"noSemicolonType": {Token: noSemicolonType, Contextual: true},
		"operatorToken":   {Token: operatorToken, Contextual: true},
		"jsx":             {Token: jsx},
	},
	Context: &lezer.ContextTracker{Start: false, Shift: trackNewline, NonStrict: true},
}

// trackNewline's context is whether a newline came since the last token
// that is not whitespace or a comment, which is where automatic semicolon
// insertion may happen.
func trackNewline(context any, term int, _ *lezer.Stack, _ *lezer.InputStream) any {
	if term == nodeLineComment || term == nodeBlockComment || term == termSpaces {
		return context
	}
	return term == termNewline
}

func afterNewline(s *lezer.Stack) bool { return s.Context() == true }

func insertSemicolon(in *lezer.InputStream, s *lezer.Stack) {
	if in.Next == braceR || in.Next == -1 || afterNewline(s) {
		in.AcceptToken(termInsertSemi, 0)
	}
}

func noSemicolon(in *lezer.InputStream, s *lezer.Stack) {
	next := in.Next
	if slices.Contains(space, next) {
		return
	}
	if next == slash {
		// Peek only here, as the original does: a peek moves the token's
		// lookahead, which decides what an edit invalidates.
		if after := in.Peek(1); after == slash || after == star {
			return
		}
	}
	if next != braceR && next != semicolon && next != -1 && !afterNewline(s) {
		in.AcceptToken(termNoSemi, 0)
	}
}

func noSemicolonType(in *lezer.InputStream, s *lezer.Stack) {
	if in.Next == bracketL && !afterNewline(s) {
		in.AcceptToken(termNoSemiType, 0)
	}
}

func operatorToken(in *lezer.InputStream, s *lezer.Stack) {
	next := in.Next
	switch {
	case next == plus || next == minus:
		in.Advance(1)
		if next != in.Next {
			return
		}
		in.Advance(1)
		token := termIncdecPrefix
		if !afterNewline(s) && s.CanShift(termIncdec) {
			token = termIncdec
		}
		in.AcceptToken(token, 0)
	case next == question && in.Peek(1) == dot:
		in.Advance(1)
		in.Advance(1)
		if in.Next < '0' || in.Next > '9' {
			in.AcceptToken(termQuestionDot, 0)
		}
	}
}

func identifierChar(ch int, start bool) bool {
	return ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch == '_' || ch >= 0xc0 ||
		!start && ch >= '0' && ch <= '9'
}

// jsx reads the < that opens a JSX tag, unless what follows is an
// identifier and a comma or extends, which makes it a type parameter list
// of an arrow function.
func jsx(in *lezer.InputStream, s *lezer.Stack) {
	if in.Next != lt || !s.DialectEnabled(dialectJsx) {
		return
	}
	in.Advance(1)
	if in.Next == slash {
		return
	}
	back := skipSpace(in)
	if identifierChar(in.Next, true) {
		in.Advance(1)
		back++
		for identifierChar(in.Next, false) {
			in.Advance(1)
			back++
		}
		back += skipSpace(in)
		if in.Next == comma {
			return
		}
		read, ok := readExtends(in)
		if !ok {
			return
		}
		back += read
	}
	in.AcceptToken(nodeJSXStartTag, -back)
}

func skipSpace(in *lezer.InputStream) int {
	n := 0
	for slices.Contains(space, in.Next) {
		in.Advance(1)
		n++
	}
	return n
}

// readExtends reads as much of the keyword extends as is there, and
// reports false when all of it is and no identifier follows: <T extends …
// opens a type parameter list.
func readExtends(in *lezer.InputStream) (read int, ok bool) {
	const keyword = "extends"
	for i := range len(keyword) {
		if in.Next != int(keyword[i]) {
			return i, true
		}
		in.Advance(1)
	}
	return len(keyword), identifierChar(in.Next, true)
}
