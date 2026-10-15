// Ported from @lezer/python 1.1.19 (src/tokens.js), MIT licensed, Copyright
// (C) 2018 by Marijn Haverbeke and others. See ../LICENSE.

package python

import "github.com/lichdotdev/godemirror/internal/lezer"

const (
	charNewline        = '\n'
	charCarriageReturn = '\r'
	charSpace          = ' '
	charTab            = '\t'
	charHash           = '#'
	charParenOpen      = '('
	charDot            = '.'
	charBraceOpen      = '{'
	charBraceClose     = '}'
	charSingleQuote    = '\''
	charDoubleQuote    = '"'
	charBackslash      = '\\'
)

// tabStop is the column multiple a tab advances indentation to, as
// Python's tokenizer counts it.
const tabStop = 8

// Flags of a context: inside brackets, and inside a string of some kind.
const (
	flagBracketed = 1 << iota
	flagString
	flagDoubleQuote
	flagLong
	flagRaw
	flagFormat
)

// bracketed are the nodes whose reduction closes a bracketed context.
var bracketed = map[int]bool{
	nodeParenthesizedExpression: true, nodeTupleExpression: true, nodeComprehensionExpression: true,
	termImportList: true, nodeArgList: true, nodeParamList: true,
	nodeArrayExpression: true, nodeArrayComprehensionExpression: true, termSubscript: true,
	nodeSetExpression: true, nodeSetComprehensionExpression: true,
	nodeFormatString: true, nodeFormatReplacement: true, termNestedFormatReplacement: true,
	nodeDictionaryExpression: true, nodeDictionaryComprehensionExpression: true,
	nodeSequencePattern: true, nodeMappingPattern: true, nodePatternArgList: true, nodeTypeParamList: true,
}

// stringFlags are the context flags each kind of string start opens.
var stringFlags = map[int]int{
	termStringStart:     flagString,
	termStringStartD:    flagString | flagDoubleQuote,
	termStringStartL:    flagString | flagLong,
	termStringStartLD:   flagString | flagLong | flagDoubleQuote,
	termStringStartR:    flagString | flagRaw,
	termStringStartRD:   flagString | flagRaw | flagDoubleQuote,
	termStringStartRL:   flagString | flagRaw | flagLong,
	termStringStartRLD:  flagString | flagRaw | flagLong | flagDoubleQuote,
	termStringStartF:    flagString | flagFormat,
	termStringStartFD:   flagString | flagFormat | flagDoubleQuote,
	termStringStartFL:   flagString | flagFormat | flagLong,
	termStringStartFLD:  flagString | flagFormat | flagLong | flagDoubleQuote,
	termStringStartFR:   flagString | flagFormat | flagRaw,
	termStringStartFRD:  flagString | flagFormat | flagRaw | flagDoubleQuote,
	termStringStartFRL:  flagString | flagFormat | flagRaw | flagLong,
	termStringStartFRLD: flagString | flagFormat | flagRaw | flagLong | flagDoubleQuote,
}

// indentContext is one level of the tracker's stack: an indented block,
// a bracket or a string. Contexts are never changed once made, so the
// parser can tell them apart by pointer.
type indentContext struct {
	parent              *indentContext
	indent, flags, hash int
}

func newContext(parent *indentContext, indent, flags int) *indentContext {
	return &indentContext{parent: parent, indent: indent, flags: flags, hash: contextHash(parent, indent, flags)}
}

// contextHash is the original's hash, JavaScript's number semantics
// included: + binds tighter than <<, and << works on 32-bit integers.
func contextHash(parent *indentContext, indent, flags int) int {
	hash := 0
	if parent != nil {
		hash = int(int32(uint32(2*parent.hash)) << 8)
	}
	return hash + indent + indent<<4 + flags + flags<<6
}

var topIndent = newContext(nil, 0, 0)

func contextOf(s *lezer.Stack) *indentContext { return s.Context().(*indentContext) }

// trackIndent keeps the stack of indented blocks, brackets and strings the
// parse is in, which decides how the tokenizers read line breaks, leading
// space and string content.
var trackIndent = lezer.ContextTracker{
	Start: topIndent,
	Reduce: func(context any, term int, _ *lezer.Stack, _ *lezer.InputStream) any {
		cx := context.(*indentContext)
		if closesContext(cx, term) {
			return cx.parent
		}
		return cx
	},
	Shift: func(context any, term int, s *lezer.Stack, in *lezer.InputStream) any {
		cx := context.(*indentContext)
		switch term {
		case termIndent:
			return newContext(cx, countIndent(in, s.Pos()), 0)
		case termDedent:
			return cx.parent
		case nodeParenL, nodeBracketL, nodeBraceL, termReplacementStart:
			return newContext(cx, 0, flagBracketed)
		}
		if flags, ok := stringFlags[term]; ok {
			return newContext(cx, 0, flags|cx.flags&flagBracketed)
		}
		return cx
	},
	Hash: func(context any) int { return context.(*indentContext).hash },
}

func closesContext(cx *indentContext, term int) bool {
	closesBracket := cx.flags&flagBracketed != 0 && bracketed[term]
	closesString := (term == nodeString || term == nodeFormatString) && cx.flags&flagString != 0
	return closesBracket || closesString
}

// countIndent is the indentation depth of the space from the stream's
// position to end.
func countIndent(in *lezer.InputStream, end int) int {
	depth := 0
	for i := range end - in.Pos {
		if in.Peek(i) == charTab {
			depth += tabStop - depth%tabStop
		} else {
			depth++
		}
	}
	return depth
}

func isLineBreak(ch int) bool { return ch == charNewline || ch == charCarriageReturn }

func isHex(ch int) bool {
	return ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'F' || ch >= 'a' && ch <= 'f'
}

// newlines reads the end of input, a line break, and the leading space of
// a blank or comment-only line, which the grammar skips. Inside brackets a
// line break is just space.
var newlines = lezer.Tokenizer{Contextual: true, Token: func(in *lezer.InputStream, s *lezer.Stack) {
	switch {
	case in.Next < 0:
		in.AcceptToken(termEof, 0)
	case contextOf(s).flags&flagBracketed != 0:
		if isLineBreak(in.Next) {
			in.AcceptToken(termNewlineBracketed, 1)
		}
	case atLineStart(in) && s.CanShift(termBlankLineStart):
		spaces := 0
		for in.Next == charSpace || in.Next == charTab {
			in.Advance(1)
			spaces++
		}
		if isLineBreak(in.Next) || in.Next == charHash {
			in.AcceptToken(termBlankLineStart, -spaces)
		}
	case isLineBreak(in.Next):
		in.AcceptToken(termNewline, 1)
	}
}}

func atLineStart(in *lezer.InputStream) bool {
	prev := in.Peek(-1)
	return prev < 0 || isLineBreak(prev)
}

// indentation reads an indent or a dedent at the start of a line whose
// depth differs from the block's, outside brackets and strings. A dedent
// ends before the line's leading space.
var indentation = lezer.Tokenizer{Token: func(in *lezer.InputStream, s *lezer.Stack) {
	cx := contextOf(s)
	if cx.flags != 0 || !isLineBreak(in.Peek(-1)) {
		return
	}
	depth, chars := 0, 0
	for ; ; chars++ {
		if in.Next == charSpace {
			depth++
		} else if in.Next == charTab {
			depth += tabStop - depth%tabStop
		} else {
			break
		}
		in.Advance(1)
	}
	if depth == cx.indent || isLineBreak(in.Next) || in.Next == charHash {
		return
	}
	if depth < cx.indent {
		in.AcceptToken(termDedent, -chars)
	} else {
		in.AcceptToken(termIndent, 0)
	}
}}

// legacyPrint reads Python 2's print statement keyword: "print" not
// followed by a word character, nor, after spaces, by "(", ".", a line
// break or a comment.
var legacyPrint = lezer.Tokenizer{Token: func(in *lezer.InputStream, _ *lezer.Stack) {
	for _, ch := range "print" {
		if in.Next != int(ch) {
			return
		}
		in.Advance(1)
	}
	if isWordChar(in.Next) {
		return
	}
	for off := 0; ; off++ {
		next := in.Peek(off)
		if next == charSpace || next == charTab {
			continue
		}
		if next != charParenOpen && next != charDot && !isLineBreak(next) && next != charHash {
			in.AcceptToken(termPrintKeyword, 0)
		}
		return
	}
}}

// isWordChar is what JavaScript's /\w/ matches.
func isWordChar(ch int) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_'
}
