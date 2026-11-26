package godemirror

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// An Editor's popups (a diagnostic's message, a hover, the completions) are
// drawn as its code is: text in cells of the editor's font, rasterized by
// glyphs, never through Gio's text stack.

const (
	// popupPadX and popupPadY are the room between a popup's edge and its
	// text, popupRadius the rounding of its corners and popupGap the room
	// between a popup and what it points at.
	popupPadX   = unit.Dp(8)
	popupPadY   = unit.Dp(4)
	popupRadius = unit.Dp(4)
	popupGap    = unit.Dp(2)
	// popupMaxCols is the widest a popup's text runs before it wraps, and
	// popupMaxRows the most rows of text a popup shows.
	popupMaxCols = 72
	popupMaxRows = 24
)

// textSpan is text a popup draws in one style.
type textSpan struct {
	text  string
	color color.NRGBA
	bold  bool
}

// popupRow is a row of a popup: spans side by side, or a rule across it.
type popupRow struct {
	spans []textSpan
	rule  bool
}

// spanImage is the image of s, made once and kept while popups draw it.
func (v *View) spanImage(s textSpan) *rowImage {
	if img, ok := v.spans[s]; ok {
		img.used = v.frameNo
		return img
	}
	img := &rowImage{}
	if cells := textCells(s.text); len(cells) > 0 {
		style := TagStyle{Color: s.color, Bold: s.bold}
		dst := v.rasterizeStyled(cells, func(int) TagStyle { return style })
		o := paint.NewImageOp(dst)
		o.Filter = paint.FilterNearest
		img = &rowImage{op: o, size: dst.Bounds().Size()}
	}
	img.used = v.frameNo
	v.spans[s] = img
	return img
}

// spansCells is how many cells spans take side by side.
func spansCells(spans []textSpan) int {
	n := 0
	for _, s := range spans {
		n += len(textCells(s.text))
	}
	return n
}

// paintTextSpans draws spans side by side from at, the top left of the first,
// clipped to area.
func (v *View) paintTextSpans(gtx layout.Context, spans []textSpan, at image.Point, area image.Rectangle) {
	for _, s := range spans {
		img := v.spanImage(s)
		v.paintImage(gtx, img, at, area)
		at.X += len(textCells(s.text)) * v.r.CellW()
	}
}

// rowHeight is how tall a popup row is: a row of text, or half of one for
// a rule.
func (r popupRow) height(ch int) int {
	if r.rule {
		return ch / 2
	}
	return ch
}

// popupSize is the size of a box holding rows.
func (e *Editor) popupSize(gtx layout.Context, rows []popupRow) image.Point {
	cols, h := 0, 0
	for _, r := range rows {
		cols = max(cols, spansCells(r.spans))
		h += r.height(e.f.ch)
	}
	return image.Pt(cols*e.f.cw+2*gtx.Dp(popupPadX), h+2*gtx.Dp(popupPadY))
}

// paintPopupBox draws a popup's box over r.
func (e *Editor) paintPopupBox(gtx layout.Context, r image.Rectangle) {
	p, radius := e.Theme.Popup, gtx.Dp(popupRadius)
	paint.FillShape(gtx.Ops, p.Border, clip.UniformRRect(r, radius).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, p.Background, clip.UniformRRect(r.Inset(1), max(radius-1, 0)).Op(gtx.Ops))
}

// paintTextPopup draws rows in a box at r.
func (e *Editor) paintTextPopup(gtx layout.Context, rows []popupRow, r image.Rectangle) {
	e.paintPopupBox(gtx, r)
	inner := r.Inset(1)
	x, y := r.Min.X+gtx.Dp(popupPadX), r.Min.Y+gtx.Dp(popupPadY)
	for _, row := range rows {
		if row.rule {
			mid := y + row.height(e.f.ch)/2
			rule := image.Rect(r.Min.X+1, mid, r.Max.X-1, mid+1)
			paint.FillShape(gtx.Ops, e.Theme.Popup.Border, clip.Rect(rule).Op())
		} else {
			e.view.paintTextSpans(gtx, row.spans, image.Pt(x, y), inner)
		}
		y += row.height(e.f.ch)
	}
}

// placePopup is where a box of size goes beside anchor, a rectangle in the
// editor: above it when above is set and there is room, else below it, and
// above again when there is room for it only there; kept inside the
// editor's width.
func (e *Editor) placePopup(gtx layout.Context, anchor image.Rectangle, size image.Point, above bool) image.Rectangle {
	gap, area := gtx.Dp(popupGap), e.f.size
	top := anchor.Min.Y - gap - size.Y
	bottom := anchor.Max.Y + gap
	fitsAbove, fitsBelow := top >= 0, bottom+size.Y <= area.Y
	y := bottom
	if above && fitsAbove || !fitsBelow && fitsAbove {
		y = top
	}
	x := min(anchor.Min.X, area.X-size.X)
	x = max(x, e.f.g.gutterW)
	return image.Rectangle{Min: image.Pt(x, y), Max: image.Pt(x+size.X, y+size.Y)}
}

// fitPopup is rows and then footer, rows cut short to fit on the side of
// anchor with the more room when the side asked for (above or not) has too
// little, and where they go. A cut drops the blank rows it leaves last.
func (e *Editor) fitPopup(gtx layout.Context, rows, footer []popupRow, anchor image.Rectangle, above bool) ([]popupRow, image.Rectangle) {
	gap := gtx.Dp(popupGap)
	room := max(anchor.Min.Y-gap, e.f.size.Y-anchor.Max.Y-gap)
	keep := min(len(rows), popupMaxRows-len(footer))
	for keep > 1 && e.popupSize(gtx, append(rows[:keep:keep], footer...)).Y > room {
		keep--
	}
	for keep > 1 && keep < len(rows) && (rows[keep-1].rule || spansCells(rows[keep-1].spans) == 0) {
		keep--
	}
	rows = append(rows[:keep:keep], footer...)
	return rows, e.placePopup(gtx, anchor, e.popupSize(gtx, rows), above)
}

// wrapText breaks text into rows of at most cols cells, at spaces where it
// can and inside a word longer than a row; each line of text starts a row.
func wrapText(text string, cols int) []string {
	var rows []string
	for line := range strings.SplitSeq(text, "\n") {
		cur := ""
		for _, word := range strings.Fields(line) {
			for len(textCells(word)) > cols {
				if cur != "" {
					rows, cur = append(rows, cur), ""
				}
				head := []rune(word)[:cols]
				rows, word = append(rows, string(head)), string([]rune(word)[cols:])
			}
			switch {
			case cur == "":
				cur = word
			case len(textCells(cur))+1+len(textCells(word)) <= cols:
				cur += " " + word
			default:
				rows, cur = append(rows, cur), word
			}
		}
		rows = append(rows, cur)
	}
	return rows
}

// textRows are text wrapped to cols, a row each, in c.
func textRows(text string, cols int, c color.NRGBA) []popupRow {
	var rows []popupRow
	for _, l := range wrapText(text, cols) {
		rows = append(rows, popupRow{spans: []textSpan{{text: l, color: c}}})
	}
	return rows
}

// popupCols is how many cells of text a popup may run to in the editor.
func (e *Editor) popupCols(gtx layout.Context) int {
	room := (e.f.size.X - e.f.g.gutterW - 2*gtx.Dp(popupPadX)) / max(e.f.cw, 1)
	return max(min(popupMaxCols, room), 1)
}
