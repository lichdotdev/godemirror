package godemirror

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// Placement is where on its line a LineWidget sits.
type Placement int

const (
	// AfterText puts the widget one cell past the line's last character, on
	// the last row the line wraps into.
	AfterText Placement = iota
	// RightEdge puts the widget against the code area's right edge, on the
	// line's first row.
	RightEdge
	// GutterCell draws the widget over the line's gutter cell, on the line's
	// first row, in place of its label: the label is not drawn while the
	// widget is. It is laid out with at most the gutter's width and one
	// row's height, and centered in the cell.
	GutterCell
)

// rightEdgeInset is a RightEdge widget's distance from the edge, 0.375rem
// at a 16px root.
const rightEdgeInset = unit.Dp(6)

// LineWidget is a Gio widget drawn on a line of the document: a button on a
// diff's hunk separator, say. It is laid out with at most the code area's
// width and one row's height. A press on it is the widget's, never the start
// of a selection.
type LineWidget struct {
	// Line is the document line (0-based) the widget sits on.
	Line      int
	Placement Placement
	// WhileHovered shows the widget only while the pointer is over its line's
	// hover group (LineDecoration.HoverGroup).
	WhileHovered bool
	// Widget is laid out once in every frame its line is on screen and
	// shown, and not at all in the others.
	Widget layout.Widget
}

func (v *View) paintWidgets(gtx layout.Context, f frame, rows []visibleRow, code image.Rectangle) {
	v.widgetAreas = v.widgetAreas[:0]
	defer clip.Rect(code).Push(gtx.Ops).Pop()
	for _, w := range v.content.Widgets {
		if w.Placement == GutterCell {
			continue
		}
		r, ok := v.shownWidgetRow(w, rows)
		if !ok {
			continue
		}
		call, dims := v.layoutWidget(gtx, w, image.Pt(code.Dx(), f.ch))
		at := image.Pt(0, r.y+(f.ch-dims.Size.Y)/2)
		if w.Placement == RightEdge {
			at.X = code.Max.X - gtx.Dp(rightEdgeInset) - dims.Size.X
		} else {
			segFrom, _ := v.segmentSpan(r.line, r.segment)
			at.X = f.g.textX + (len(v.doc.lines[r.line])-segFrom+1)*f.cw
			if !v.Wrap {
				at.X -= v.scroll.X
			}
		}
		v.placeWidget(gtx, call, at, dims)
	}
}

// paintGutterWidgets draws the GutterCell widgets over the gutter, which is
// already painted, centered in their cells.
func (v *View) paintGutterWidgets(gtx layout.Context, f frame, rows []visibleRow) {
	defer clip.Rect(image.Rect(0, 0, f.g.gutterW, f.size.Y)).Push(gtx.Ops).Pop()
	for _, w := range v.content.Widgets {
		if w.Placement != GutterCell {
			continue
		}
		r, ok := v.shownWidgetRow(w, rows)
		if !ok {
			continue
		}
		call, dims := v.layoutWidget(gtx, w, image.Pt(f.g.gutterW, f.ch))
		at := image.Pt(max((f.g.gutterW-dims.Size.X)/2, 0), r.y+(f.ch-dims.Size.Y)/2)
		v.placeWidget(gtx, call, at, dims)
	}
}

// coveredLabels are the lines whose gutter label a GutterCell widget
// replaces this frame.
func (v *View) coveredLabels(rows []visibleRow) map[int]bool {
	var covered map[int]bool
	for _, w := range v.content.Widgets {
		if _, ok := v.shownWidgetRow(w, rows); ok && w.Placement == GutterCell {
			if covered == nil {
				covered = map[int]bool{}
			}
			covered[w.Line] = true
		}
	}
	return covered
}

// shownWidgetRow is the row w is drawn on, if it is on screen and not
// waiting for a hover.
func (v *View) shownWidgetRow(w LineWidget, rows []visibleRow) (visibleRow, bool) {
	if w.WhileHovered && (v.hot == 0 || v.decoration(w.Line).HoverGroup != v.hot) {
		return visibleRow{}, false
	}
	return v.widgetRow(w, rows)
}

// layoutWidget lays w out once, within limit, and records it to be placed.
func (v *View) layoutWidget(gtx layout.Context, w LineWidget, limit image.Point) (op.CallOp, layout.Dimensions) {
	wgtx := gtx
	wgtx.Constraints = layout.Constraints{Max: limit}
	macro := op.Record(gtx.Ops)
	dims := w.Widget(wgtx)
	return macro.Stop(), dims
}

// placeWidget draws a laid-out widget at at, and remembers where, so a press
// there is the widget's.
func (v *View) placeWidget(gtx layout.Context, call op.CallOp, at image.Point, dims layout.Dimensions) {
	v.widgetAreas = append(v.widgetAreas, image.Rectangle{Min: at, Max: at.Add(dims.Size)})
	t := op.Offset(at).Push(gtx.Ops)
	call.Add(gtx.Ops)
	t.Pop()
}

// widgetRow is the visible row w sits on, if it is on screen.
func (v *View) widgetRow(w LineWidget, rows []visibleRow) (visibleRow, bool) {
	if w.Line < 0 || w.Line >= len(v.doc.lines) {
		return visibleRow{}, false
	}
	segment := 0
	if w.Placement == AfterText {
		segment = len(v.wraps[w.Line]) - 1
	}
	for _, r := range rows {
		if r.line == w.Line && r.segment == segment {
			return r, true
		}
	}
	return visibleRow{}, false
}

func (v *View) onWidget(p image.Point) bool {
	for _, a := range v.widgetAreas {
		if p.In(a) {
			return true
		}
	}
	return false
}
