package godemirror

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// defaultIndentUnit is CodeMirror's indentUnit when none is set
// (@codemirror/language 6.13.1).
const defaultIndentUnit = "  "

var errIndentUnit = errors.New("godemirror: IndentUnit must be one or more spaces, or one or more tabs")

// indentUnitOf is unit, or the default when unit is empty, failing as
// CodeMirror's indentUnit facet does for anything but a run of one
// whitespace character repeated.
func indentUnitOf(unit string) (string, error) {
	if unit == "" {
		return defaultIndentUnit, nil
	}
	first, _ := utf8.DecodeRuneInString(unit)
	for _, r := range unit {
		if !allJSSpace(string(r)) || r != first {
			return "", errIndentUnit
		}
	}
	return unit, nil
}

// indentWidth is the width of the indent unit in columns, as
// @codemirror/language's getIndentUnit.
func (s EditorState) indentWidth() int {
	n := utf8.RuneCountInString(s.conf.IndentUnit)
	if s.conf.IndentUnit[0] == '\t' {
		return tabSize * n
	}
	return n
}

// indentString is whitespace covering cols columns, tabs first when the
// indent unit is tabs, as @codemirror/language's indentString.
func (s EditorState) indentString(cols int) string {
	var b strings.Builder
	ch, _ := utf8.DecodeRuneInString(s.conf.IndentUnit)
	if ch == '\t' {
		for ; cols >= tabSize; cols -= tabSize {
			b.WriteByte('\t')
		}
		ch = ' '
	}
	for range cols {
		b.WriteRune(ch)
	}
	return b.String()
}

// leadingSpace is the run of whitespace, as JavaScript's \s matches it, that
// starts s.
func leadingSpace(s string) string {
	for i, r := range s {
		if !allJSSpace(string(r)) {
			return s[:i]
		}
	}
	return s
}
