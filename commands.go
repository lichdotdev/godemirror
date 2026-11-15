package godemirror

import (
	"regexp"
	"unicode/utf16"
)

// The commands in this file port @codemirror/commands 6.11.1's that need
// nothing but the state, and the cursor motion of @codemirror/view 6.43.15's
// moveByChar and moveByGroup, which needs no layout here: text runs left to
// right in one direction, so a line is one bidi span. Each returns the
// transaction it makes, ok false when it does not apply, as a CodeMirror
// command returns false.

// rangeEnd is a cursor at the end of r a motion goes to.
func rangeEnd(r SelectionRange, forward bool) SelectionRange {
	if forward {
		return Cursor(r.to)
	}
	return Cursor(r.from)
}

func (s EditorState) updateSel(by func(SelectionRange) SelectionRange) EditorSelection {
	ranges := make([]SelectionRange, len(s.sel.ranges))
	for i, r := range s.sel.ranges {
		ranges[i] = by(r)
	}
	return NewSelection(ranges, s.sel.main)
}

func setSel(sel EditorSelection) TransactionSpec {
	return TransactionSpec{Selection: sel, ScrollIntoView: true, UserEvent: "select"}
}

// moveSel moves every range with how.
func (s EditorState) moveSel(how func(SelectionRange) SelectionRange) (TransactionSpec, bool) {
	sel := s.updateSel(how)
	if sel.eq(s.sel, true) {
		return TransactionSpec{}, false
	}
	return setSel(sel), true
}

// extendSel moves the head of every range with how, keeping its anchor.
func (s EditorState) extendSel(how func(SelectionRange) SelectionRange) (TransactionSpec, bool) {
	sel := s.updateSel(func(r SelectionRange) SelectionRange {
		head := how(r)
		return rangeWith(r.Anchor(), head.Head(), head.goal, head.Assoc())
	})
	if sel.eq(s.sel, false) {
		return TransactionSpec{}, false
	}
	return setSel(sel), true
}

// moveVisually is @codemirror/view's moveVisually for a line that is one
// left-to-right span: the cursor one cluster on from start within line, or
// ok false at the line's end. moved is the text it moved over.
func moveVisually(line Line, start SelectionRange, forward bool) (next SelectionRange, moved string, ok bool) {
	n := len(line.Text)
	if n == 0 {
		return SelectionRange{}, "", false
	}
	index := start.Head() - line.From
	switch {
	case start.Head() == line.From && start.Assoc() < 0:
		if !forward {
			return SelectionRange{}, "", false
		}
		index = 0
	case start.Head() == line.To && start.Assoc() > 0:
		if forward {
			return SelectionRange{}, "", false
		}
		index = n
	}
	end := 0
	if forward {
		end = n
	}
	if index == end {
		return SelectionRange{}, "", false
	}
	nextIndex := findClusterBreak(line.Text, index, forward, true)
	moved = line.Text[min(index, nextIndex):max(index, nextIndex)]
	switch {
	case nextIndex == end && forward:
		return cursorAssoc(line.To, 1), moved, true
	case nextIndex == end:
		return cursorAssoc(line.From, -1), moved, true
	case forward:
		return cursorAssoc(nextIndex+line.From, -1), moved, true
	}
	return cursorAssoc(nextIndex+line.From, 1), moved, true
}

// moveByChar is @codemirror/view's moveByChar: start moved one cluster, or
// across a line break, on as long as by, given the first text moved over,
// accepts the next.
func (s EditorState) moveByChar(start SelectionRange, forward bool, by func(initial string) func(next string) bool) SelectionRange {
	line := s.LineAt(start.Head())
	var check func(string) bool
	for cur := start; ; {
		next, char, ok := moveVisually(line, cur, forward)
		if !ok {
			if forward && line.Number == s.Lines() || !forward && line.Number == 1 {
				return cur
			}
			char = "\n"
			if forward {
				line = s.Line(line.Number + 1)
				next = cursorAssoc(line.From, -1)
			} else {
				line = s.Line(line.Number - 1)
				next = cursorAssoc(line.To, 1)
			}
		}
		if check == nil {
			if by == nil {
				return next
			}
			check = by(char)
		} else if !check(char) {
			return cur
		}
		cur = next
	}
}

// moveByGroup moves across a run of word characters, or of other
// non-space ones, and the spaces before it.
func (s EditorState) moveByGroup(start SelectionRange, forward bool) SelectionRange {
	return s.moveByChar(start, forward, func(initial string) func(string) bool {
		cat := categorize(initial)
		return func(next string) bool {
			nextCat := categorize(next)
			if cat == categorySpace {
				cat = nextCat
			}
			return cat == nextCat
		}
	})
}

func (s EditorState) cursorByChar(forward bool) (TransactionSpec, bool) {
	return s.moveSel(func(r SelectionRange) SelectionRange {
		if r.Empty() {
			return s.moveByChar(r, forward, nil)
		}
		return rangeEnd(r, forward)
	})
}

func (s EditorState) selectByChar(forward bool) (TransactionSpec, bool) {
	return s.extendSel(func(r SelectionRange) SelectionRange { return s.moveByChar(r, forward, nil) })
}

func (s EditorState) cursorByGroup(forward bool) (TransactionSpec, bool) {
	return s.moveSel(func(r SelectionRange) SelectionRange {
		if r.Empty() {
			return s.moveByGroup(r, forward)
		}
		return rangeEnd(r, forward)
	})
}

func (s EditorState) selectByGroup(forward bool) (TransactionSpec, bool) {
	return s.extendSel(func(r SelectionRange) SelectionRange { return s.moveByGroup(r, forward) })
}

func (s EditorState) cursorDoc(end bool) (TransactionSpec, bool) {
	pos := 0
	if end {
		pos = len(s.doc)
	}
	return setSel(SingleSelection(pos, pos)), true
}

func (s EditorState) selectDoc(end bool) (TransactionSpec, bool) {
	head := 0
	if end {
		head = len(s.doc)
	}
	return setSel(SingleSelection(s.sel.Main().Anchor(), head)), true
}

func (s EditorState) selectAll() (TransactionSpec, bool) {
	return TransactionSpec{Selection: SingleSelection(0, len(s.doc)), UserEvent: "select"}, true
}

// deleteBy deletes every range, or, for a cursor, up to where by puts it.
func (s EditorState) deleteBy(by func(SelectionRange) int) (TransactionSpec, bool) {
	event := "delete.selection"
	spec := s.ChangeByRange(func(r SelectionRange) RangeChange {
		from, to := r.from, r.to
		if from == to {
			towards := by(r)
			if towards < from {
				event = "delete.backward"
			} else if towards > from {
				event = "delete.forward"
			}
			from, to = min(from, towards), max(to, towards)
		}
		if from == to {
			return RangeChange{Range: r}
		}
		assoc := 1
		if from < r.Head() {
			assoc = -1
		}
		return RangeChange{Changes: []ChangeSpec{{From: from, To: to}}, Range: cursorAssoc(from, assoc)}
	})
	if spec.err == nil && spec.set.Empty() {
		return TransactionSpec{}, false
	}
	spec.UserEvent, spec.ScrollIntoView = event, true
	return spec, true
}

var nonBlank = regexp.MustCompile(`[^ \t]`)

// deleteByChar deletes a cluster, or a line break, before or after each
// cursor. Backward, byIndentUnit deletes the spaces of indentation back to
// the previous indent stop.
func (s EditorState) deleteByChar(forward, byIndentUnit bool) (TransactionSpec, bool) {
	return s.deleteBy(func(r SelectionRange) int {
		pos := r.from
		line := s.LineAt(pos)
		if byIndentUnit && !forward && pos > line.From && pos < line.From+200 && !nonBlank.MatchString(line.Text[:pos-line.From]) {
			before := line.Text[:pos-line.From]
			if before[len(before)-1] == '\t' {
				return pos - 1
			}
			col := countColumn(before, tabSize, len(before))
			drop := col % s.indentWidth()
			if drop == 0 {
				drop = s.indentWidth()
			}
			for i := 0; i < drop && i < len(before) && before[len(before)-1-i] == ' '; i++ {
				pos--
			}
			return pos
		}
		target := findClusterBreak(line.Text, pos-line.From, forward, forward) + line.From
		last := 1
		if forward {
			last = s.Lines()
		}
		if target == pos && line.Number != last {
			if forward {
				return target + 1
			}
			return target - 1
		}
		if !forward && hasVariationSelector(line.Text[target-line.From:pos-line.From]) {
			target = findClusterBreak(line.Text, target-line.From, false, false) + line.From
		}
		return target
	})
}

func hasVariationSelector(s string) bool {
	for _, r := range s {
		if r >= 0xfe00 && r <= 0xfe0f {
			return true
		}
	}
	return false
}

// deleteByGroup deletes a group of word characters, or of other non-space
// ones, before or after each cursor, with the spaces next to it.
func (s EditorState) deleteByGroup(forward bool) (TransactionSpec, bool) {
	return s.deleteBy(func(r SelectionRange) int {
		pos := r.Head()
		line := s.LineAt(pos)
		lineEnd, last := line.From, 1
		if forward {
			lineEnd, last = line.To, s.Lines()
		}
		cat, haveCat := categoryWord, false
		for {
			if pos == lineEnd {
				if pos == r.Head() && line.Number != last {
					if forward {
						pos++
					} else {
						pos--
					}
				}
				return pos
			}
			next := findClusterBreak(line.Text, pos-line.From, forward, true) + line.From
			nextChar := line.Text[min(pos, next)-line.From : max(pos, next)-line.From]
			nextCat := categorize(nextChar)
			if haveCat && nextCat != cat {
				return pos
			}
			if nextChar != " " || pos != r.Head() {
				cat, haveCat = nextCat, true
			}
			pos = next
		}
	})
}

// insertNewlineAndIndent replaces each range with a line break and the
// indentation of its line, as CodeMirror does with no language to indent
// by: there, only a line break at the very start of the document is
// indented by the syntax tree, an empty one, to column 0. Between a pair of
// brackets the line breaks twice and the cursor sits on the middle line.
func (s EditorState) insertNewlineAndIndent() (TransactionSpec, bool) {
	spec := s.ChangeByRange(func(r SelectionRange) RangeChange {
		from, to := r.from, r.to
		line := s.LineAt(from)
		explode := from == to && s.betweenBrackets(from)
		indent := 0
		if from > 0 {
			space := leadingSpace(s.LineAt(from).Text)
			indent = countColumn(space, tabSize, len(space))
		}
		for to < line.To && allJSSpace(line.Text[to-line.From:runeEnd(line.Text, to-line.From)]) {
			to = line.From + runeEnd(line.Text, to-line.From)
		}
		// CodeMirror tests the line's text up to from as a position in the
		// document, not in the line, so past the document's first lines it
		// looks at the whole line: kept, as it decides when the spaces
		// before the cursor are dropped.
		if explode {
			from, to = r.from, r.from
		} else if from > line.From && from < line.From+100 && !hasNonSpace(utf16Prefix(line.Text, utf16Len(s.doc[:from]))) {
			from = line.From
		}
		insert := "\n" + s.indentString(indent)
		if explode {
			// The indentation of the line before the break, as
			// IndentContext.lineIndent sees it with the break simulated.
			before := line.Text[:from-line.From]
			insert += "\n" + s.indentString(countColumn(before, tabSize, len(leadingSpace(before))))
		}
		return RangeChange{
			Changes: []ChangeSpec{{From: from, To: to, Insert: insert}},
			Range:   Cursor(from + 1 + len(s.indentString(indent))),
		}
	})
	spec.UserEvent, spec.ScrollIntoView = "input", true
	return spec, true
}

// betweenBrackets reports whether pos sits between "()", "[]" or "{}".
func (s EditorState) betweenBrackets(pos int) bool {
	if pos < 1 || pos+1 > len(s.doc) {
		return false
	}
	switch s.doc[pos-1 : pos+1] {
	case "()", "[]", "{}":
		return true
	}
	return false
}

func hasNonSpace(s string) bool { return !allJSSpace(s) }

// utf16Prefix is the start of s up to n UTF-16 code units, as JavaScript's
// s.slice(0, n).
func utf16Prefix(s string, n int) string {
	for i, r := range s {
		if n <= 0 {
			return s[:i]
		}
		n -= utf16.RuneLen(r)
	}
	return s
}

// groupAt is the range a double click at pos selects, as @codemirror/view's
// groupAt: the run of characters of one category around it, on the side
// bias says when pos sits between two runs.
func (s EditorState) groupAt(pos, bias int) SelectionRange {
	line := s.LineAt(pos)
	linePos := pos - line.From
	if len(line.Text) == 0 {
		return Cursor(pos)
	}
	if linePos == 0 {
		bias = 1
	} else if linePos == len(line.Text) {
		bias = -1
	}
	from, to := linePos, linePos
	if bias < 0 {
		from = findClusterBreak(line.Text, linePos, false, true)
	} else {
		to = findClusterBreak(line.Text, linePos, true, true)
	}
	cat := categorize(line.Text[from:to])
	for from > 0 {
		prev := findClusterBreak(line.Text, from, false, true)
		if categorize(line.Text[prev:from]) != cat {
			break
		}
		from = prev
	}
	for to < len(line.Text) {
		next := findClusterBreak(line.Text, to, true, true)
		if categorize(line.Text[to:next]) != cat {
			break
		}
		to = next
	}
	return Range(from+line.From, to+line.From)
}
