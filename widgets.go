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
		if w.WhileHovered && (v.hot == 0 || v.decoration(w.Line).HoverGroup != v.hot) {
			continue
		}
		r, ok := v.widgetRow(w, rows)
		if !ok {
			continue
		}
		wgtx := gtx
		wgtx.Constraints = layout.Constraints{Max: image.Pt(code.Dx(), f.ch)}
		macro := op.Record(gtx.Ops)
		dims := w.Widget(wgtx)
		call := macro.Stop()
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
		area := image.Rectangle{Min: at, Max: at.Add(dims.Size)}
		v.widgetAreas = append(v.widgetAreas, area)
		t := op.Offset(at).Push(gtx.Ops)
		call.Add(gtx.Ops)
		t.Pop()
	}
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
