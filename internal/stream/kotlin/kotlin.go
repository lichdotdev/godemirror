// Ported from @codemirror/legacy-modes 6.5.5 (mode/clike.js: clike and its
// kotlin configuration), MIT licensed, Copyright (C) 2018-2021 by Marijn
// Haverbeke and others. See ../LICENSE.

// Package kotlin is the Kotlin mode of @codemirror/legacy-modes, the one
// CodeMirror highlights Kotlin with.
//
// Only what decides a token's style is ported: clike's contexts, alignment
// and typeAtEndOfLine feed indentation alone, and none of them is kept.
package kotlin

import (
	"strings"
	"unicode/utf16"

	"github.com/lichdotdev/godemirror/internal/stream"
)

var (
	keywords = words("package as typealias class interface this super val operator " +
		"var fun for is in This throw return annotation " +
		"break continue object if else while do try when !in !is as? " +
		"file import where by get set abstract enum open inner override private public internal " +
		"protected catch finally out final vararg reified dynamic companion constructor init " +
		"sealed field property receiver param sparam lateinit data inline noinline tailrec " +
		"external annotation crossinline const operator infix suspend actual expect setparam")
	types = words("Boolean Byte Character CharSequence Class ClassLoader Cloneable Comparable " +
		"Compiler Double Exception Float Integer Long Math Number Object Package Pair Process " +
		"Runtime Runnable SecurityManager Short StackTraceElement StrictMath String " +
		"StringBuffer System Thread ThreadGroup ThreadLocal Throwable Triple Void Annotation Any BooleanArray " +
		"ByteArray Char CharArray DeprecationLevel DoubleArray Enum FloatArray Function Int IntArray Lazy " +
		"LazyThreadSafetyMode LongArray Nothing ShortArray Unit")
	defKeywords = words("class val var object interface fun")
	atoms       = words("true false null this")
)

func words(list string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Split(list, " ") {
		set[w] = true
	}
	return set
}

// Mode is the state of the Kotlin mode over one document.
type Mode struct {
	tokenize     func(*stream.Stream) string
	prevToken    string
	curPunc      string
	isDefKeyword bool
}

// New is the Kotlin mode at the start of a document.
func New() stream.Mode { return &Mode{} }

// Token is clike's token, less what only indentation reads.
func (m *Mode) Token(s *stream.Stream) string {
	if s.EatSpace() {
		return ""
	}
	m.curPunc, m.isDefKeyword = "", false
	tokenize := m.tokenize
	if tokenize == nil {
		tokenize = m.tokenBase
	}
	style := tokenize(s)
	if style == "comment" || style == "meta" {
		return style
	}
	if style == "variable" && m.prevToken == "def" {
		style = "def"
	}
	switch {
	case m.isDefKeyword:
		m.prevToken = "def"
	case style != "":
		m.prevToken = style
	default:
		m.prevToken = m.curPunc
	}
	return style
}

func (m *Mode) tokenBase(s *stream.Stream) string {
	ch := s.Next()
	if style, ok := m.hook(s, ch); ok {
		return style
	}
	if ch == '\'' {
		m.tokenize = m.tokenString(uint16(ch))
		return m.tokenize(s)
	}
	if ch == '.' || stream.IsDigit(uint16(ch)) {
		s.BackUp(1)
		if s.MatchFunc(number) {
			return "number"
		}
		s.Next()
	}
	if isPunctuationChar(ch) {
		m.curPunc = string(rune(ch))
		return ""
	}
	if ch == '/' && s.Eat('/') {
		s.SkipToEnd()
		return "comment"
	}
	if isOperatorChar(uint16(ch)) {
		for !s.LookingAt("//") && !s.LookingAt("/*") && s.EatFunc(isOperatorChar) {
		}
		return "operator"
	}
	s.EatWhile(isIdentifierChar)
	return m.word(string(utf16.Decode(s.Current())))
}

// hook is kotlin's hooks for the character just read, ok false where
// clike goes on as if there were none. The "/" hook takes only "/*", so a
// "/*" never reaches clike's own block comment.
func (m *Mode) hook(s *stream.Stream, ch int) (string, bool) {
	switch ch {
	case '@':
		s.EatWhile(func(u uint16) bool { return stream.IsWord(u) || u == '$' })
		return "meta", true
	case '*':
		if m.prevToken == "." {
			return "variable", true
		}
		return "operator", true
	case '"':
		m.tokenize = m.tokenKotlinString(s.Match(`""`))
		return m.tokenize(s), true
	case '/':
		if !s.Eat('*') {
			return "", false
		}
		m.tokenize = m.tokenNestedComment(1)
		return m.tokenize(s), true
	}
	return "", false
}

func (m *Mode) word(cur string) string {
	if keywords[cur] {
		m.isDefKeyword = defKeywords[cur]
		return "keyword"
	}
	if types[cur] {
		return "type"
	}
	if atoms[cur] {
		return "atom"
	}
	return "variable"
}

// tokenString is clike's, with multiLineStrings: a string the line does not
// close goes on into the next.
func (m *Mode) tokenString(quote uint16) func(*stream.Stream) string {
	return func(s *stream.Stream) string {
		escaped := false
		for next := s.Next(); next != stream.EOL; next = s.Next() {
			if next == int(quote) && !escaped {
				m.tokenize = nil
				break
			}
			escaped = !escaped && next == '\\'
		}
		return "string"
	}
}

func (m *Mode) tokenKotlinString(tripleString bool) func(*stream.Stream) string {
	return func(s *stream.Stream) string {
		escaped, end := false, false
		for !s.EOL() {
			if !tripleString && !escaped && s.Match(`"`) || tripleString && s.Match(`"""`) {
				end = true
				break
			}
			next := s.Next()
			if !escaped && next == '$' && s.Match("{") {
				s.SkipTo('}')
			}
			escaped = !escaped && next == '\\' && !tripleString
		}
		if end || !tripleString {
			m.tokenize = nil
		}
		return "string"
	}
}

func (m *Mode) tokenNestedComment(depth int) func(*stream.Stream) string {
	return func(s *stream.Stream) string {
		for ch := s.Next(); ch != stream.EOL; ch = s.Next() {
			if ch == '*' && s.Eat('/') {
				if depth == 1 {
					m.tokenize = nil
					break
				}
				m.tokenize = m.tokenNestedComment(depth - 1)
				return m.tokenize(s)
			}
			if ch == '/' && s.Eat('*') {
				m.tokenize = m.tokenNestedComment(depth + 1)
				return m.tokenize(s)
			}
		}
		return "comment"
	}
}

// isPunctuationChar is clike's /[\[\]{}\(\),;\:\.]/.
func isPunctuationChar(ch int) bool {
	return ch >= 0 && strings.ContainsRune("[]{}(),;:.", rune(ch))
}

// isOperatorChar is clike's /[+\-*&%=<>!?|\/]/.
func isOperatorChar(u uint16) bool { return strings.ContainsRune("+-*&%=<>!?|/", rune(u)) }

// isIdentifierChar is clike's /[\w\$_\xa1-￿]/.
func isIdentifierChar(u uint16) bool { return stream.IsWord(u) || u == '$' || u >= 0xa1 }
