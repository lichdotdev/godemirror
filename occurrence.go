package godemirror

import "strings"

// selectNextOccurrence and selectSelectionMatches port @codemirror/search
// 6.7.2's commands of the same names, with its SearchCursor reduced to what
// they search for: text equal, code unit for code unit, to the selection.
// CodeMirror's cursor also matches text equal once both are NFKD-normalized
// (a precomposed "é" and "e" with a combining accent); godemirror's does not.

// maxSelectionMatches is how many matches selectSelectionMatches selects
// before it gives up, as @codemirror/search's does.
const maxSelectionMatches = 1000

// wordAt is the run of word characters around pos, as CodeMirror's
// EditorState.wordAt, ok false when none touches it.
func (s EditorState) wordAt(pos int) (SelectionRange, bool) {
	line := s.LineAt(pos)
	start, end := pos-line.From, pos-line.From
	for start > 0 {
		prev := findClusterBreak(line.Text, start, false, true)
		if categorize(line.Text[prev:start]) != categoryWord {
			break
		}
		start = prev
	}
	for end < len(line.Text) {
		next := findClusterBreak(line.Text, end, true, true)
		if categorize(line.Text[end:next]) != categoryWord {
			break
		}
		end = next
	}
	if start == end {
		return SelectionRange{}, false
	}
	return Range(start+line.From, end+line.From), true
}

// matchesIn are the places query appears in the document's bytes [from, to),
// in order, none overlapping the one before, as SearchCursor.next finds them.
func (s EditorState) matchesIn(query string, from, to int) func(yield func(from, to int) bool) {
	return func(yield func(from, to int) bool) {
		if query == "" {
			return
		}
		for at := from; at < to; {
			i := strings.Index(s.doc[at:to], query)
			if i < 0 {
				return
			}
			if !yield(at+i, at+i+len(query)) {
				return
			}
			at += i + len(query)
		}
	}
}

// selectWord selects the word around each cursor.
func (s EditorState) selectWord() (TransactionSpec, bool) {
	sel := s.updateSel(func(r SelectionRange) SelectionRange {
		if w, ok := s.wordAt(r.Head()); ok {
			return w
		}
		return Cursor(r.Head())
	})
	if sel.eq(s.sel, false) {
		return TransactionSpec{}, false
	}
	return TransactionSpec{Selection: sel}, true
}

// findNextOccurrence is the next place query appears after the last range,
// wrapping around to the start, as a whole word when the main range is one,
// ok false when there is none.
func (s EditorState) findNextOccurrence(query string) (from, to int, ok bool) {
	main, ranges := s.sel.Main(), s.sel.ranges
	last := ranges[len(ranges)-1]
	word, isWord := s.wordAt(main.Head())
	fullWord := isWord && word.from == main.from && word.to == main.to
	accept := func(from, to int, cycled bool) bool {
		if cycled && hasFrom(ranges, from) {
			return false
		}
		if fullWord {
			w, ok := s.wordAt(from)
			return ok && w.from == from && w.to == to
		}
		return true
	}
	for from, to := range s.matchesIn(query, last.to, len(s.doc)) {
		if accept(from, to, false) {
			return from, to, true
		}
	}
	// The wrapped search stops a character short of the last range, as
	// CodeMirror's stops a code unit short of it.
	end := 0
	if last.from > 0 {
		end = prevRuneStart(s.doc, last.from)
	}
	for from, to := range s.matchesIn(query, 0, end) {
		if accept(from, to, true) {
			return from, to, true
		}
	}
	return 0, 0, false
}

func hasFrom(ranges []SelectionRange, from int) bool {
	for _, r := range ranges {
		if r.from == from {
			return true
		}
	}
	return false
}

// selectNextOccurrence adds the next place the selected text appears to the
// selection, or, with a cursor among the ranges, selects the words around
// the cursors.
func (s EditorState) selectNextOccurrence() (TransactionSpec, bool) {
	for _, r := range s.sel.ranges {
		if r.Empty() {
			return s.selectWord()
		}
	}
	// CodeMirror takes the query with the line separator, and searches the
	// document's lines joined by "\n": kept, so with LineSeparator "\r\n" a
	// selection over a line break is found nowhere else.
	query := s.SliceDoc(s.sel.ranges[0].from, s.sel.ranges[0].to)
	for _, r := range s.sel.ranges {
		if s.SliceDoc(r.from, r.to) != query {
			return TransactionSpec{}, false
		}
	}
	from, to, ok := s.findNextOccurrence(query)
	if !ok {
		return TransactionSpec{}, false
	}
	return TransactionSpec{Selection: s.sel.addSecondary(Range(from, to)), scrollTo: &to}, true
}

// selectSelectionMatches selects every place the one selected range's text
// appears, the range itself main, or nothing past maxSelectionMatches.
func (s EditorState) selectSelectionMatches() (TransactionSpec, bool) {
	main := s.sel.Main()
	if len(s.sel.ranges) > 1 || main.Empty() {
		return TransactionSpec{}, false
	}
	var ranges []SelectionRange
	mainIndex := 0
	for from, to := range s.matchesIn(s.SliceDoc(main.from, main.to), 0, len(s.doc)) {
		if len(ranges) > maxSelectionMatches {
			return TransactionSpec{}, false
		}
		if from == main.from {
			mainIndex = len(ranges)
		}
		ranges = append(ranges, Range(from, to))
	}
	return TransactionSpec{Selection: NewSelection(ranges, mainIndex), UserEvent: "select.search.matches"}, true
}
