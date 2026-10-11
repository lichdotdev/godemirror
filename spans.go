package godemirror

import (
	"image"
	"image/color"
	"slices"

	"gioui.org/layout"
)

// Span is a stretch of a line's characters, From to To in rune columns
// (0-based, To excluded, a tab or a wide character one column), and the Color it is tinted with.
type Span struct {
	From, To int
	Color    color.NRGBA
}

// spanCells is the cells [from, to) s covers on line, clamped to the line.
func (v *View) spanCells(line int, s Span) (from, to int) {
	cellOf, end := v.doc.cellOf[line], len(v.doc.lines[line])
	cell := func(col int) int {
		if col >= len(cellOf) {
			return end
		}
		return cellOf[max(col, 0)]
	}
	return cell(s.From), cell(s.To)
}

// paintSpans tints the spans of every visible row over the row's background,
// and the search matches after them. They are painted beneath the text like
// the selection, so the row images never hold them.
func (v *View) paintSpans(gtx layout.Context, f frame, rows []visibleRow, code image.Rectangle) {
	for _, r := range rows {
		d := v.decoration(r.line)
		spans := d.Spans
		if found := v.searchSpans(r.line); len(found) > 0 {
			spans = slices.Concat(spans, found)
		}
		if len(spans) == 0 {
			continue
		}
		base := v.Theme.Background
		if bg := v.background(d); bg.A != 0 {
			base = over(bg, base)
		}
		segFrom, segTo := v.segmentSpan(r.line, r.segment)
		x := f.g.textX - segFrom*f.cw
		if !v.Wrap {
			x -= v.scroll.X
		}
		for _, s := range spans {
			from, to := v.spanCells(r.line, s)
			from, to = max(from, segFrom), min(to, segTo)
			if from >= to {
				continue
			}
			cells := image.Rect(x+from*f.cw, r.y, x+to*f.cw, r.y+f.ch).Intersect(code)
			fill(gtx, base, s.Color, cells)
		}
	}
}
