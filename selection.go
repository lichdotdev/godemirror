package godemirror

import (
	"image"
	"slices"
	"strings"
	"unicode"

	"gioui.org/layout"
)

// pos is a place between two cells: line, and the cell col sits before.
type pos struct{ line, col int }

func (p pos) before(o pos) bool {
	return p.line < o.line || p.line == o.line && p.col < o.col
}

// selection is a span from where it was started (anchor) to where it is now
// (head); it is empty when the two meet.
type selection struct{ anchor, head pos }

func (s selection) empty() bool { return s.anchor == s.head }

func (s selection) ordered() (from, to pos) {
	if s.head.before(s.anchor) {
		return s.head, s.anchor
	}
	return s.anchor, s.head
}

// SelectedText is the selected text as the document had it, tabs and all,
// or "" when nothing is selected.
func (v *View) SelectedText() string {
	if v.sel.empty() {
		return ""
	}
	from, to := v.sel.ordered()
	var b strings.Builder
	for line := from.line; line <= to.line; line++ {
		start, end := 0, len(v.doc.lines[line])
		if line == from.line {
			start = from.col
		}
		if line == to.line {
			end = to.col
		}
		if line > from.line {
			b.WriteByte('\n')
		}
		b.WriteString(v.doc.sourceText(line, start, end))
	}
	return b.String()
}

// SelectedLines is the span of document lines (0-based, inclusive) the
// selection covers. A selection that ends at the very start of a line does
// not take that line in. ok is false when nothing is selected.
func (v *View) SelectedLines() (from, to int, ok bool) {
	if v.sel.empty() {
		return 0, 0, false
	}
	start, end := v.sel.ordered()
	to = end.line
	if end.col == 0 && end.line > start.line {
		to--
	}
	return start.line, to, true
}

// ClearSelection drops the selection.
func (v *View) ClearSelection() { v.sel = selection{} }

// Pos is a place in the document between two characters: Line is 0-based,
// and Col counts the runes of the line's text before it, a tab as one.
type Pos struct{ Line, Col int }

// Select selects the text between from and to, in either order, as a drag
// between them would. A position past the document's start or end, or past
// its line's end, is clamped to it. The view does not scroll to it.
func (v *View) Select(from, to Pos) {
	v.sel = selection{v.cellPos(from), v.cellPos(to)}
}

// SelectLines selects lines from to to (0-based, inclusive) whole, with the
// newline after the last of them, as a triple click selects one line; it is
// the span SelectedLines then reports. Lines past the document are clamped.
func (v *View) SelectLines(from, to int) {
	last := len(v.doc.lines) - 1
	from, to = min(max(from, 0), last), min(max(to, 0), last)
	start := v.lineAt(pos{line: min(from, to)})
	end := v.lineAt(pos{line: max(from, to)})
	v.sel = selection{start.anchor, end.head}
}

// Selection is the selection's start and end, from before to, or ok false
// when nothing is selected. A selection made with the pointer that starts or
// ends inside a tab is read as starting or ending after it.
func (v *View) Selection() (from, to Pos, ok bool) {
	if v.sel.empty() {
		return Pos{}, Pos{}, false
	}
	start, end := v.sel.ordered()
	return v.runePos(start), v.runePos(end), true
}

// cellPos is the place between cells p names, clamped to the document.
func (v *View) cellPos(p Pos) pos {
	last := len(v.doc.lines) - 1
	switch {
	case p.Line < 0:
		return pos{}
	case p.Line > last:
		return pos{last, len(v.doc.lines[last])}
	}
	cells := v.doc.cellOf[p.Line]
	if p.Col >= len(cells) {
		return pos{p.Line, len(v.doc.lines[p.Line])}
	}
	return pos{p.Line, cells[max(p.Col, 0)]}
}

// runePos is p counted in runes: the first rune that starts at or after it.
func (v *View) runePos(p pos) Pos {
	col, _ := slices.BinarySearch(v.doc.cellOf[p.line], p.col)
	return Pos{p.line, col}
}

// hit is the place between cells nearest to p, a point in the view. Above
// the document it is the start, below it the end, and over a block the end of
// the line the block opens under.
func (v *View) hit(f frame, p image.Point) pos {
	y := v.scroll.Y + p.Y
	line, segment, ok := locate(v.tops, v.starts, f.ch, y)
	switch {
	case !ok && y < 0:
		return pos{}
	case !ok:
		last := len(v.doc.lines) - 1
		return pos{last, len(v.doc.lines[last])}
	case segment < 0:
		return pos{line, len(v.doc.lines[line])}
	}
	x := p.X - f.g.textX + f.cw/2
	if !v.Wrap {
		x += v.scroll.X
	}
	from, to := v.segmentSpan(line, segment)
	return pos{line, from + min(max(x/f.cw, 0), to-from)}
}

// segmentSpan is the cells [from, to) the row segment of line shows. A row
// past the line's own wrapping (padding in a split) shows none, at its end.
func (v *View) segmentSpan(line, segment int) (from, to int) {
	end := len(v.doc.lines[line])
	wraps := v.wraps[line]
	if segment >= len(wraps) {
		return end, end
	}
	from, to = wraps[segment], end
	if segment+1 < len(wraps) {
		to = wraps[segment+1]
	}
	return from, to
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordAt is the span a double click at p selects: the run of word characters
// around it, or, off a word, the single cell under it.
func (v *View) wordAt(p pos) selection {
	cells := v.doc.lines[p.line]
	if len(cells) == 0 {
		return selection{p, p}
	}
	at := min(p.col, len(cells)-1)
	if !isWordRune(cells[at]) && at > 0 && isWordRune(cells[at-1]) {
		at--
	}
	if !isWordRune(cells[at]) {
		return selection{pos{p.line, at}, pos{p.line, at + 1}}
	}
	from, to := at, at+1
	for from > 0 && isWordRune(cells[from-1]) {
		from--
	}
	for to < len(cells) && isWordRune(cells[to]) {
		to++
	}
	return selection{pos{p.line, from}, pos{p.line, to}}
}

// lineAt is the span a triple click selects: the whole line and its newline.
func (v *View) lineAt(p pos) selection {
	if p.line+1 < len(v.doc.lines) {
		return selection{pos{p.line, 0}, pos{p.line + 1, 0}}
	}
	return selection{pos{p.line, 0}, pos{p.line, len(v.doc.lines[p.line])}}
}

// paintSelection fills the selected cells of every visible row. A line the
// selection runs past also has its newline lit, one cell beyond its end. A
// selection that is exactly a search match is the current match, which
// paintSpans already lit in the theme's SearchCurrent.
func (v *View) paintSelection(gtx layout.Context, f frame, rows []visibleRow, code image.Rectangle) {
	if v.sel.empty() || v.isCurrentMatch() {
		return
	}
	from, to := v.sel.ordered()
	for _, r := range rows {
		if r.line < from.line || r.line > to.line {
			continue
		}
		start, end := 0, len(v.doc.lines[r.line])+1
		if r.line == from.line {
			start = from.col
		}
		if r.line == to.line {
			end = to.col
		}
		segFrom, segTo := v.segmentSpan(r.line, r.segment)
		if segTo == len(v.doc.lines[r.line]) {
			segTo++
		}
		start, end = max(start, segFrom), min(end, segTo)
		if start >= end {
			continue
		}
		x := f.g.textX - segFrom*f.cw
		if !v.Wrap {
			x -= v.scroll.X
		}
		cells := image.Rect(x+start*f.cw, r.y, x+end*f.cw, r.y+f.ch).Intersect(code)
		fill(gtx, v.Theme.Background, v.Theme.Selection, cells)
	}
}

// selectPress starts or extends the selection for a press at p, the clicks-th
// in a row at the same place.
func (v *View) selectPress(p pos, clicks int, extend bool) {
	switch {
	case extend:
		v.sel.head = p
	case clicks == 2:
		v.sel = v.wordAt(p)
	case clicks >= 3:
		v.sel = v.lineAt(p)
	default:
		v.sel = selection{p, p}
	}
}
