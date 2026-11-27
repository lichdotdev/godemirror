package godemirror

import "strings"

// Indentation by brackets (BracketIndent) is not a port: CodeMirror leaves
// indentation to a language, and the Lezer ports carry none.

// opensBracket reports whether text leaves a bracket open: one closed
// before any opened on the line, as in "} else {", does not count. Brackets
// in strings and comments count as any other.
func opensBracket(text string) bool {
	depth := 0
	for _, r := range text {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth = max(depth-1, 0)
		}
	}
	return depth > 0
}

// outdentClosing is change, text typed, with the indentation before it taken
// one IndentUnit back when it is a closing bracket typed first on its line
// and BracketIndent is set; and where the cursor goes after it.
func (s EditorState) outdentClosing(change ChangeSpec) (ChangeSpec, int) {
	end := change.From + len(change.Insert)
	if !s.conf.BracketIndent || change.From != change.To || len(change.Insert) != 1 || !strings.Contains(")]}", change.Insert) {
		return change, end
	}
	line := s.LineAt(change.From)
	before := line.Text[:change.From-line.From]
	if before == "" || leadingSpace(before) != before {
		return change, end
	}
	indent := s.indentString(max(countColumn(before, tabSize, len(before))-s.indentWidth(), 0))
	out := ChangeSpec{From: line.From, To: change.From, Insert: indent + change.Insert}
	return out, line.From + len(out.Insert)
}
