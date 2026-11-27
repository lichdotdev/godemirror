package godemirror

import "slices"

// Tab and Shift+Tab (IndentWithTab) follow CodeMirror's indentWithTab, with
// insertTab's behavior for a cursor: a unit at the cursor, not at the line's
// start.

// indentByTab is what Tab does, or Shift+Tab when back: an IndentUnit
// inserted at each cursor when no range selects anything, else every line a
// range is on indented one unit more, or one less when back.
func (s EditorState) indentByTab(back bool) TransactionSpec {
	if !back && !slices.ContainsFunc(s.sel.ranges, func(r SelectionRange) bool { return !r.Empty() }) {
		spec := s.ReplaceSelection(s.conf.IndentUnit)
		spec.UserEvent, spec.ScrollIntoView = "input.indent", true
		return spec
	}
	unit := s.indentWidth()
	var changes []ChangeSpec
	last := -1
	for _, r := range s.sel.ranges {
		first, end := s.LineAt(r.from).Number, s.LineAt(r.to)
		final := end.Number
		if r.to > r.from && r.to == end.From {
			final-- // a range ending at a line's start leaves that line be
		}
		for n := max(first, last+1); n <= final; n++ {
			l := s.Line(n)
			space := leadingSpace(l.Text)
			cols := countColumn(space, tabSize, len(space))
			if back {
				cols = max(cols-unit, 0)
			} else {
				cols += unit
			}
			if indent := s.indentString(cols); indent != space {
				changes = append(changes, ChangeSpec{From: l.From, To: l.From + len(space), Insert: indent})
			}
		}
		last = max(last, final)
	}
	return TransactionSpec{Changes: changes, UserEvent: "input.indent", ScrollIntoView: true}
}
