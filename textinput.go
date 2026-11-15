package godemirror

import (
	"strings"
	"unicode/utf16"
)

// applyDefaultInsert ports @codemirror/view 6.43.15's function of the same
// name (domchange.ts), without atomic ranges. change is what the input
// method replaced, newSel where it left the main cursor (-1 for nowhere it
// said), and composing whether a composition is under way.
func (s EditorState) applyDefaultInsert(change ChangeSpec, newSel int, composing bool) TransactionSpec {
	main := s.sel.Main()
	insert := normalizeBreaks(change.Insert, s.conf.LineSeparator)
	var spec TransactionSpec
	switch {
	case change.From >= main.from && change.To <= main.to &&
		3*utf16Len(s.doc[change.From:change.To]) >= utf16Len(s.doc[main.from:main.to]) &&
		(newSel < 0 || newSel == change.From+len(insert)) && !composing:
		before, after := "", ""
		if main.from < change.From {
			before = s.SliceDoc(main.from, change.From)
		}
		if main.to > change.To {
			after = s.SliceDoc(change.To, main.to)
		}
		spec = s.ReplaceSelection(before + s.withLineBreaks(insert) + after)
	case len(s.sel.ranges) > 1 && composing && change.To <= main.to+10 && change.To >= main.to-10:
		spec = s.composeAtEveryCursor(change, insert, newSel)
	default:
		spec = TransactionSpec{Changes: []ChangeSpec{change}}
		if newSel >= 0 && newSel <= len(s.doc)-(change.To-change.From)+len(insert) {
			spec.Selection = s.keepOtherRanges(change, Cursor(newSel))
		}
	}
	spec.UserEvent, spec.ScrollIntoView = "input.type", true
	if composing {
		spec.UserEvent += ".compose"
	}
	return spec
}

// withLineBreaks is text, its lines joined by "\n", joined by the line
// separator instead, as Text.sliceString gives it with the state's line
// break.
func (s EditorState) withLineBreaks(text string) string {
	if sep := s.lineBreak(); sep != "\n" {
		return strings.ReplaceAll(text, "\n", sep)
	}
	return text
}

// composeAtEveryCursor repeats a composition's change at the main cursor at
// every other cursor that has the same text before it, outside the line of
// the main cursor, as CodeMirror does with no DOM node to find the
// composition in.
func (s EditorState) composeAtEveryCursor(change ChangeSpec, insert string, newSel int) TransactionSpec {
	main := s.sel.Main()
	changes, err := s.changes([]ChangeSpec{change})
	if err != nil {
		return TransactionSpec{err: err}
	}
	hasMainSel := newSel >= 0 && newSel <= changes.NewLen()
	replaced := s.doc[change.From:change.To]
	composition := s.LineAt(main.Head())
	offset := main.to - change.To
	return s.ChangeByRange(func(r SelectionRange) RangeChange {
		if r.from == main.from && r.to == main.to {
			if hasMainSel {
				return RangeChange{Changes: []ChangeSpec{change}, Range: Cursor(newSel)}
			}
			return RangeChange{Changes: []ChangeSpec{change}, Range: r.Map(changes)}
		}
		to := r.to - offset
		from := to - len(replaced)
		if from < 0 || to > len(s.doc) || s.doc[from:to] != replaced || to >= composition.From && from <= composition.To {
			return RangeChange{Range: r}
		}
		own := ChangeSpec{From: from, To: to, Insert: insert}
		if !hasMainSel {
			rangeChanges, err := s.changes([]ChangeSpec{own})
			if err != nil {
				return RangeChange{Range: r}
			}
			return RangeChange{Changes: []ChangeSpec{own}, Range: r.Map(rangeChanges)}
		}
		off := r.to - main.to
		return RangeChange{Changes: []ChangeSpec{own}, Range: Range(max(0, newSel+off), max(0, newSel+off))}
	})
}

// keepOtherRanges is the selection with main as its main range and the
// others left at the offsets they had before change, unmapped, as
// CodeMirror leaves them. Its offsets count UTF-16 code units, so the ranges
// keep their code units in the new document, not their bytes; one beyond
// the new document makes the transaction fail, as CodeMirror's throws.
func (s EditorState) keepOtherRanges(change ChangeSpec, main SelectionRange) EditorSelection {
	after := s.doc[:change.From] + change.Insert + s.doc[change.To:]
	ranges := make([]SelectionRange, len(s.sel.ranges))
	for i, r := range s.sel.ranges {
		if i == s.sel.main {
			ranges[i] = main
			continue
		}
		anchor := byteAtUnit(after, utf16Len(s.doc[:r.Anchor()]))
		head := byteAtUnit(after, utf16Len(s.doc[:r.Head()]))
		ranges[i] = rangeWith(anchor, head, r.goal, r.Assoc())
	}
	return NewSelection(ranges, s.sel.main)
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// byteAtUnit is the byte offset of s at unit UTF-16 code units, the start
// of the character a unit inside one falls in, or len(s)+1 past its end.
func byteAtUnit(s string, unit int) int {
	n := 0
	for i, r := range s {
		next := n + utf16.RuneLen(r)
		if unit < next {
			return i
		}
		n = next
	}
	if unit == n {
		return len(s)
	}
	return len(s) + 1
}
