package godemirror

import (
	"cmp"
	"slices"
)

// SelectionRange and EditorSelection port @codemirror/state 6.7.6's
// selection.ts, without the bidi level of a cursor and the undirectional
// ranges of macOS, which nothing in godemirror reads.

// rangeFlags are what a range keeps beside its bounds.
type rangeFlags uint8

const (
	assocBefore rangeFlags = 1 << iota
	assocAfter
	inverted
)

// noGoal is the goal column of a range that has none.
const noGoal = -1

// SelectionRange is one range of an EditorSelection, from its anchor, the
// side that stays put when the range is extended, to its head. Positions are
// byte offsets into EditorState.Doc. Build one with Cursor or Range.
type SelectionRange struct {
	from, to int
	flags    rangeFlags
	// goal is the column a cursor moving up or down keeps aiming for, in
	// cells (CodeMirror keeps pixels; a cell is a monospace character's
	// width), or noGoal.
	goal int
}

// Cursor is an empty range at pos.
func Cursor(pos int) SelectionRange { return cursorAssoc(pos, 0) }

func cursorAssoc(pos, assoc int) SelectionRange {
	return SelectionRange{from: pos, to: pos, flags: assocFlag(assoc), goal: noGoal}
}

// Range is the range from anchor to head, in either order.
func Range(anchor, head int) SelectionRange { return rangeWith(anchor, head, noGoal, 0) }

func rangeWith(anchor, head, goal, assoc int) SelectionRange {
	if assoc == 0 && anchor != head {
		assoc = -1
		if head < anchor {
			assoc = 1
		}
	}
	flags := assocFlag(assoc)
	if head < anchor {
		return SelectionRange{from: head, to: anchor, flags: flags | inverted, goal: goal}
	}
	return SelectionRange{from: anchor, to: head, flags: flags, goal: goal}
}

func assocFlag(assoc int) rangeFlags {
	switch {
	case assoc < 0:
		return assocBefore
	case assoc > 0:
		return assocAfter
	}
	return 0
}

// From is the lower of the range's bounds.
func (r SelectionRange) From() int { return r.from }

// To is the upper of the range's bounds.
func (r SelectionRange) To() int { return r.to }

// Anchor is the side of the range that stays put when it is extended.
func (r SelectionRange) Anchor() int {
	if r.flags&inverted != 0 {
		return r.to
	}
	return r.from
}

// Head is the side of the range that moves when it is extended.
func (r SelectionRange) Head() int {
	if r.flags&inverted != 0 {
		return r.from
	}
	return r.to
}

// Empty reports whether the range is a cursor: its anchor is its head.
func (r SelectionRange) Empty() bool { return r.from == r.to }

// Assoc is the side a cursor sticks to: -1 the character before it, 1 the one
// after, 0 neither.
func (r SelectionRange) Assoc() int {
	switch {
	case r.flags&assocBefore != 0:
		return -1
	case r.flags&assocAfter != 0:
		return 1
	}
	return 0
}

// GoalColumn is the column, in cells, a cursor moving up or down keeps aiming
// for across shorter lines, ok false when it has none.
func (r SelectionRange) GoalColumn() (col int, ok bool) { return r.goal, r.goal != noGoal }

// eq is CodeMirror's SelectionRange.eq: the same anchor, head and goal
// column, and, with includeAssoc, a cursor sticking to the same side.
func (r SelectionRange) eq(o SelectionRange, includeAssoc bool) bool {
	return r.Anchor() == o.Anchor() && r.Head() == o.Head() && r.goal == o.goal &&
		(!includeAssoc || !r.Empty() || r.Assoc() == o.Assoc())
}

// Map is the range moved through changes, which start from the document
// it is in: a cursor keeps to the character before it.
func (r SelectionRange) Map(c ChangeSet) SelectionRange { return r.mapAssoc(c, -1) }

func (r SelectionRange) mapAssoc(c ChangeSet, assoc int) SelectionRange {
	var from, to int
	if r.Empty() {
		from = c.MapPos(r.from, assoc)
		to = from
	} else {
		from = c.MapPos(r.from, 1)
		to = c.MapPos(r.to, -1)
	}
	if from == r.from && to == r.to {
		return r
	}
	return SelectionRange{from: from, to: to, flags: r.flags, goal: r.goal}
}

// Extend is the range grown to cover at least [from, to], keeping its anchor
// unless that falls inside.
func (r SelectionRange) Extend(from, to int) SelectionRange { return r.extend(from, to, 0) }

func (r SelectionRange) extend(from, to, assoc int) SelectionRange {
	anchor := r.Anchor()
	if from <= anchor && to >= anchor {
		return rangeWith(from, to, noGoal, assoc)
	}
	head := to
	if abs(from-anchor) > abs(to-anchor) {
		head = from
	}
	return rangeWith(anchor, head, noGoal, assoc)
}

// EditorSelection is one or more ranges, sorted and never overlapping (two
// non-empty ranges may touch), one of them the main range, as CodeMirror's
// EditorSelection. Its zero value has no ranges: a TransactionSpec or
// EditorStateConfig takes it as no selection given.
type EditorSelection struct {
	ranges []SelectionRange
	main   int
}

// NewSelection is a selection of ranges with ranges[main] as its main range.
// Ranges are sorted, and those that overlap merged, as CodeMirror's
// EditorSelection.create does: two cursors at one place become one, and a
// range merged into another takes the direction of the later of the two. It
// panics when ranges is empty or main is not one of its indices.
func NewSelection(ranges []SelectionRange, main int) EditorSelection {
	if len(ranges) == 0 {
		panic("godemirror: a selection needs at least one range")
	}
	if main < 0 || main >= len(ranges) {
		panic("godemirror: the main range of a selection is not one of its ranges")
	}
	pos := 0
	for _, r := range ranges {
		if r.Empty() && r.from <= pos || !r.Empty() && r.from < pos {
			return normalized(slices.Clone(ranges), main)
		}
		pos = r.to
	}
	return EditorSelection{slices.Clone(ranges), main}
}

// SingleSelection is a selection of the one range from anchor to head.
func SingleSelection(anchor, head int) EditorSelection {
	return EditorSelection{[]SelectionRange{Range(anchor, head)}, 0}
}

func normalized(ranges []SelectionRange, main int) EditorSelection {
	order := make([]int, len(ranges))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(ranges[a].from, ranges[b].from) })
	sorted := make([]SelectionRange, len(ranges))
	wasMain := main
	for i, o := range order {
		sorted[i] = ranges[o]
		if o == wasMain {
			main = i
		}
	}
	for i := 1; i < len(sorted); i++ {
		r, prev := sorted[i], sorted[i-1]
		if r.Empty() && r.from <= prev.to || !r.Empty() && r.from < prev.to {
			from, to := prev.from, max(r.to, prev.to)
			if i <= main {
				main--
			}
			merged := Range(from, to)
			if r.Anchor() > r.Head() {
				merged = Range(to, from)
			}
			sorted = slices.Replace(sorted, i-1, i+1, merged)
			i--
		}
	}
	return EditorSelection{sorted, main}
}

// Ranges are the selection's ranges in document order. It is a copy.
func (s EditorSelection) Ranges() []SelectionRange { return slices.Clone(s.ranges) }

// MainIndex is the index of the main range in Ranges.
func (s EditorSelection) MainIndex() int { return s.main }

// Main is the main range, usually the one added last.
func (s EditorSelection) Main() SelectionRange { return s.ranges[s.main] }

// Map is the selection moved through changes, which start from the
// document it is in: a cursor keeps to the character before it.
func (s EditorSelection) Map(c ChangeSet) EditorSelection {
	if c.Empty() {
		return s
	}
	ranges := make([]SelectionRange, len(s.ranges))
	for i, r := range s.ranges {
		ranges[i] = r.Map(c)
	}
	return NewSelection(ranges, s.main)
}

// AsSingle is the selection of the main range alone.
func (s EditorSelection) AsSingle() EditorSelection {
	if len(s.ranges) == 1 {
		return s
	}
	return EditorSelection{[]SelectionRange{s.Main()}, 0}
}

// AddRange is the selection with r added as its main range, merged with any
// range it overlaps.
func (s EditorSelection) AddRange(r SelectionRange) EditorSelection {
	return NewSelection(append([]SelectionRange{r}, s.ranges...), 0)
}

// eq is CodeMirror's EditorSelection.eq: the same ranges, by
// SelectionRange.eq, and the same main range.
func (s EditorSelection) eq(o EditorSelection, includeAssoc bool) bool {
	if len(s.ranges) != len(o.ranges) || s.main != o.main {
		return false
	}
	for i, r := range s.ranges {
		if !r.eq(o.ranges[i], includeAssoc) {
			return false
		}
	}
	return true
}

// ReplaceRange is the selection with its range at index which replaced by r,
// merged with any range it then overlaps.
func (s EditorSelection) ReplaceRange(r SelectionRange, which int) EditorSelection {
	ranges := slices.Clone(s.ranges)
	ranges[which] = r
	return NewSelection(ranges, s.main)
}
