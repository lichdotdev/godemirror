// Ported from @lezer/css 1.3.9 (src/tokens.js), MIT licensed, Copyright (C)
// 2018 by Marijn Haverbeke and others. See internal/lezer/LICENSE.

package css

import (
	"regexp"
	"slices"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// space is the code units tokens.js counts as whitespace before a token.
var space = []int{9, 10, 11, 12, 13, 32, 133, 160, 5760, 8192, 8193, 8194, 8195, 8196, 8197,
	8198, 8199, 8200, 8201, 8202, 8232, 8233, 8239, 8287, 12288}

const (
	colon      = ':'
	parenL     = '('
	underscore = '_'
	bracketL   = '['
	dash       = '-'
	period     = '.'
	hash       = '#'
	percent    = '%'
	ampersand  = '&'
	backslash  = '\\'
	newline    = '\n'
	asterisk   = '*'
	blank      = ' '
	// firstNonASCIILetter is where tokens.js starts taking every code unit
	// as a letter: everything from U+00A1 up, surrogates included.
	firstNonASCIILetter = 161
)

func isAlpha(ch int) bool {
	return ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= firstNonASCIILetter
}

func isDigit(ch int) bool { return ch >= '0' && ch <= '9' }

func isHex(ch int) bool { return isDigit(ch) || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F' }

func isSpace(ch int) bool { return slices.Contains(space, ch) }

// identifierTokens reads an identifier, a CSS variable name (two leading
// dashes, where the parser can take one) or a callee (followed by "(").
func identifierTokens(id, varName, callee int) func(*lezer.InputStream, *lezer.Stack) {
	return func(in *lezer.InputStream, s *lezer.Stack) {
		inside, dashes := false, 0
		for i := 0; ; i++ {
			next := in.Next
			switch {
			case isAlpha(next) || next == dash || next == underscore || (inside && isDigit(next)):
				if !inside && (next != dash || i > 0) {
					inside = true
				}
				if dashes == i && next == dash {
					dashes++
				}
				in.Advance(1)
			case next == backslash && in.Peek(1) != newline:
				readEscape(in)
				inside = true
			default:
				if inside {
					in.AcceptToken(identifierKind(in, s, dashes, id, varName, callee), 0)
				}
				return
			}
		}
	}
}

// readEscape reads a backslash escape: hex digits and the space ending
// them, or the one code unit escaped.
func readEscape(in *lezer.InputStream) {
	in.Advance(1)
	if !isHex(in.Next) {
		if in.Next > -1 {
			in.Advance(1)
		}
		return
	}
	for isHex(in.Advance(1)) {
	}
	if in.Next == blank {
		in.Advance(1)
	}
}

func identifierKind(in *lezer.InputStream, s *lezer.Stack, dashes, id, varName, callee int) int {
	switch {
	case dashes >= 2 && s.CanShift(nodeVariableName):
		return varName
	case in.Next == parenL:
		return callee
	default:
		return id
	}
}

func descendant(in *lezer.InputStream, _ *lezer.Stack) {
	if !isSpace(in.Peek(-1)) {
		return
	}
	if startsSelector(in) {
		in.AcceptToken(termDescendantOp, 0)
	}
}

func startsSelector(in *lezer.InputStream) bool {
	switch next := in.Next; next {
	case underscore, hash, period, asterisk, bracketL, dash, ampersand:
		return true
	case colon:
		return isAlpha(in.Peek(1))
	default:
		return isAlpha(next)
	}
}

func unitToken(in *lezer.InputStream, _ *lezer.Stack) {
	if isSpace(in.Peek(-1)) {
		return
	}
	next := in.Next
	if next == percent {
		in.Advance(1)
		in.AcceptToken(nodeUnit, 0)
	}
	if isAlpha(next) {
		for {
			in.Advance(1)
			if !isAlpha(in.Next) && !isDigit(in.Next) {
				break
			}
		}
		in.AcceptToken(nodeUnit, 0)
	}
}

var hexColor = regexp.MustCompile(`^#[a-fA-F0-9]+$`)

// hashColor specializes a hash name that reads as a hex color (#rgb,
// #rgba, #rrggbb, #rrggbbaa) to hashNameColor. The length is in bytes where
// tokens.js counts UTF-16 units, which only differ for names the pattern
// rejects anyway.
func hashColor(name string, _ *lezer.Stack) int {
	switch len(name) {
	case 4, 5, 7, 9:
		if hexColor.MatchString(name) {
			return termHashNameColor
		}
	}
	return -1
}

var externals = lezer.Externals{
	Tokenizers: map[string]lezer.Tokenizer{
		"descendant":       {Token: descendant},
		"unitToken":        {Token: unitToken},
		"identifiers":      {Token: identifierTokens(termIdentifier, nodeVariableName, termCallee), Contextual: true},
		"queryIdentifiers": {Token: identifierTokens(termQueryIdentifier, termQueryVariableName, nodeQueryCallee), Contextual: true},
	},
	Specializers: map[string]lezer.Specializer{"hashColor": hashColor},
}
