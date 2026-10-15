// Ported from @codemirror/legacy-modes 6.5.5 (mode/shell.js), MIT licensed,
// Copyright (C) 2018-2021 by Marijn Haverbeke and others. See ../LICENSE.

// Package shell is the shell mode of @codemirror/legacy-modes, the one
// CodeMirror highlights shell scripts with.
package shell

import (
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/lichdotdev/godemirror/internal/stream"
)

var styleOfWord = map[string]string{}

func init() {
	define("atom", "true false")
	define("keyword", "if then do else elif while until for in esac fi "+
		"fin fil done exit set unset export function")
	define("builtin", "ab awk bash beep cat cc cd chown chmod chroot clear "+
		"cp curl cut diff echo find gawk gcc get git grep hg kill killall "+
		"ln ls make mkdir openssl mv nc nl node npm ping ps restart rm "+
		"rmdir sed service sh shopt shred source sort sleep ssh start stop "+
		"su sudo svn tee telnet top touch vi vim wall wc wget who write "+
		"yes zsh")
}

func define(style, list string) {
	for _, w := range strings.Fields(list) {
		styleOfWord[w] = style
	}
}

type tokenizer func(*stream.Stream) string

// Mode is the state of the shell mode over one document: shell.js's stack
// of tokenizers, its top last where shell.js keeps it first.
type Mode struct {
	tokens []tokenizer
}

// New is the shell mode at the start of a document.
func New() stream.Mode { return &Mode{} }

// Token is shell.js's token.
func (m *Mode) Token(s *stream.Stream) string { return m.tokenize(s) }

func (m *Mode) tokenize(s *stream.Stream) string {
	if len(m.tokens) == 0 {
		return m.tokenBase(s)
	}
	return m.tokens[len(m.tokens)-1](s)
}

func (m *Mode) push(t tokenizer)       { m.tokens = append(m.tokens, t) }
func (m *Mode) pop()                   { m.tokens = m.tokens[:len(m.tokens)-1] }
func (m *Mode) replaceTop(t tokenizer) { m.tokens[len(m.tokens)-1] = t }

func (m *Mode) tokenBase(s *stream.Stream) string {
	if s.EatSpace() {
		return ""
	}
	sol := s.SOL()
	ch := s.Next()
	switch ch {
	case '\\':
		s.Next()
		return ""
	case '\'', '"', '`':
		style := "string"
		if ch == '`' {
			style = "quote"
		}
		m.push(m.tokenString(uint16(ch), style))
		return m.tokenize(s)
	case '#':
		if sol && s.Eat('!') {
			s.SkipToEnd()
			return "meta"
		}
		s.SkipToEnd()
		return "comment"
	case '$':
		m.push(m.tokenDollar)
		return m.tokenize(s)
	case '+', '=':
		return "operator"
	case '-':
		s.Eat('-')
		s.EatWhile(stream.IsWord)
		return "attribute"
	case '<':
		if style, ok := m.redirection(s); ok {
			return style
		}
	}
	return m.word(s, ch)
}

// redirection reads what follows a "<": "<<" more makes "<<<", and else a
// heredoc's opening, whose delimiter may be empty. A "<" followed by
// anything else is no redirection, ok false.
func (m *Mode) redirection(s *stream.Stream) (string, bool) {
	if s.Match("<<") {
		return "operator", true
	}
	var delim []uint16
	if !s.MatchFunc(func(rest []uint16) int {
		n, d := heredoc(rest)
		delim = d
		return n
	}) {
		return "", false
	}
	m.push(m.tokenHeredoc(delim))
	return "string.special", true
}

// word is the end of tokenBase: a number, or a word that may be a
// definition or one of the words shell.js knows.
func (m *Mode) word(s *stream.Stream, ch int) string {
	if ch >= 0 && stream.IsDigit(uint16(ch)) {
		s.EatWhile(stream.IsDigit)
		if s.EOL() || !stream.IsWord(uint16(s.Peek())) {
			return "number"
		}
	}
	s.EatWhile(func(u uint16) bool { return stream.IsWord(u) || u == '-' })
	cur := s.Current()
	if s.Peek() == '=' && slices.ContainsFunc(cur, stream.IsWord) {
		return "def"
	}
	return styleOfWord[string(utf16.Decode(cur))]
}

// heredoc matches /^<-?\s*(?:['"]([^'"]*)['"]|([^'"\s]*))/ at the start of
// rest: its length, -1 for none, and the delimiter it captured. The second
// alternative matches the empty string, so the match never backtracks into
// the first part.
func heredoc(rest []uint16) (int, []uint16) {
	if len(rest) == 0 || rest[0] != '<' {
		return -1, nil
	}
	n := 1
	if n < len(rest) && rest[n] == '-' {
		n++
	}
	for n < len(rest) && stream.IsSpace(rest[n]) {
		n++
	}
	if n < len(rest) && isQuote(rest[n]) {
		if end := slices.IndexFunc(rest[n+1:], isQuote); end >= 0 {
			return n + 1 + end + 1, rest[n+1 : n+1+end]
		}
	}
	start := n
	for n < len(rest) && !isQuote(rest[n]) && !stream.IsSpace(rest[n]) {
		n++
	}
	return n, rest[start:n]
}

func (m *Mode) tokenString(quote uint16, style string) tokenizer {
	closing := quote
	switch quote {
	case '(':
		closing = ')'
	case '{':
		closing = '}'
	}
	return func(s *stream.Stream) string {
		escaped := false
		for next := s.Next(); next != stream.EOL; next = s.Next() {
			switch {
			case next == int(closing) && !escaped:
				m.pop()
				return style
			case next == '$' && !escaped && quote != '\'' && s.Peek() != int(closing):
				s.BackUp(1)
				m.push(m.tokenDollar)
				return style
			case !escaped && quote != closing && next == int(quote):
				m.push(m.tokenString(quote, style))
				return m.tokenize(s)
			case !escaped && isQuote(uint16(next)) && !isQuote(quote):
				m.push(m.tokenStringStart(uint16(next), "string"))
				s.BackUp(1)
				return style
			}
			escaped = !escaped && next == '\\'
		}
		return style
	}
}

func (m *Mode) tokenStringStart(quote uint16, style string) tokenizer {
	return func(s *stream.Stream) string {
		m.replaceTop(m.tokenString(quote, style))
		s.Next()
		return m.tokenize(s)
	}
}

func (m *Mode) tokenDollar(s *stream.Stream) string {
	if len(m.tokens) > 1 {
		s.Eat('$')
	}
	ch := s.Next()
	switch ch {
	case '\'', '"', '(', '{':
		style := "string"
		switch ch {
		case '(':
			style = "quote"
		case '{':
			style = "def"
		}
		m.replaceTop(m.tokenString(uint16(ch), style))
		return m.tokenize(s)
	}
	if ch < 0 || !stream.IsDigit(uint16(ch)) {
		s.EatWhile(stream.IsWord)
	}
	m.pop()
	return "def"
}

// tokenHeredoc styles every line up to the one that is the delimiter
// alone, that line and the rest of the line that opened it included.
func (m *Mode) tokenHeredoc(delim []uint16) tokenizer {
	return func(s *stream.Stream) string {
		if s.SOL() && slices.Equal(s.Line(), delim) {
			m.pop()
		}
		s.SkipToEnd()
		return "string.special"
	}
}

func isQuote(u uint16) bool { return u == '\'' || u == '"' }
