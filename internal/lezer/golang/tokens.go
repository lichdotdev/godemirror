// Ported from @lezer/go 1.0.1 (src/tokens.js), MIT licensed, Copyright (C)
// 2018 by Marijn Haverbeke and others. See ../LICENSE.

package golang

import "github.com/lichdotdev/godemirror/internal/lezer"

const (
	charNewline        = '\n'
	charCarriageReturn = '\r'
	charSpace          = ' '
	charTab            = '\t'
	charSlash          = '/'
	charCloseParen     = ')'
	charCloseBrace     = '}'
)

// semicolon inserts Go's automatic semicolon: after a line that ends in a
// token the tracker marks, before a newline, a line comment or the end, and
// always before a ) or }. Like the original, it accepts at the first
// position where that holds, scanning over spaces and tabs.
var semicolon = lezer.Tokenizer{Contextual: true, Token: func(in *lezer.InputStream, s *lezer.Stack) {
	for scan, next := 0, in.Next; ; {
		if insertsSemicolon(s.Context() == true, next, in.Peek(scan+1)) {
			in.AcceptToken(termInsertedSemi, 0)
		}
		if next != charSpace && next != charTab {
			return
		}
		scan++
		next = in.Peek(scan)
	}
}}

func insertsSemicolon(afterTrackedToken bool, next, after int) bool {
	if next == charCloseParen || next == charCloseBrace {
		return true
	}
	lineEnds := next < 0 || next == charNewline || next == charCarriageReturn ||
		next == charSlash && after == charSlash
	return afterTrackedToken && lineEnds
}

var trackedTokens = map[int]bool{
	nodeIncDecOp: true, termIdentifier: true, nodeRune: true, nodeString: true, nodeNumber: true,
	termBreak: true, termContinue: true, termReturn: true, termFallthrough: true,
	termCloseParen: true, termCloseBracket: true, termCloseBrace: true,
}

// trackTokens is whether the last token shifted, spaces aside, is one
// after which a line break ends the statement.
var trackTokens = lezer.ContextTracker{
	Start: false,
	Shift: func(context any, term int, _ *lezer.Stack, _ *lezer.InputStream) any {
		if term == termSpace {
			return context
		}
		return trackedTokens[term]
	},
}
