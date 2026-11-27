package godemirror

import (
	"image"
	"image/color"

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

// spansCells is how many cells spans take laid in a row.
func spansCells(spans []textSpan) int {
	n := 0
	for _, s := range spans {
		n += len(textCells(s.text))
	}
	return n
}

// paintTextSpans draws spans one after another from at, the top left of the first,
// clipped to area.
func (v *View) paintTextSpans(gtx layout.Context, spans []textSpan, at image.Point, area image.Rectangle) {
	for _, s := range spans {
		img := v.spanImage(s)
		v.paintImage(gtx, img, at, area)
		at.X += len(textCells(s.text)) * v.r.CellW()
	}
}

// paintPopupBox draws a popup's box over r.
func (e *Editor) paintPopupBox(gtx layout.Context, r image.Rectangle) {
	p, radius := e.Theme.Popup, gtx.Dp(popupRadius)
	paint.FillShape(gtx.Ops, p.Border, clip.UniformRRect(r, radius).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, p.Background, clip.UniformRRect(r.Inset(1), max(radius-1, 0)).Op(gtx.Ops))
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

// maxProseWidth is the widest a popup's text runs in the editor.
func (e *Editor) maxProseWidth(gtx layout.Context) int {
	return max(min(gtx.Dp(popupMaxWidth), e.f.size.X-e.f.g.gutterW-2*gtx.Dp(popupPadX)-2), e.f.cw)
}

// proseBox lays blocks out at most maxW wide in a popup of kind, placed by
// place given its size (an empty rectangle draws nothing), no taller than
// popupMaxHeight nor the editor; its content scrolls inside it.
func (e *Editor) proseBox(gtx layout.Context, kind int, blocks []mdBlock, maxW int, place func(size image.Point) image.Rectangle) {
	p := e.layoutProse(gtx, blocks, maxW)
	padX, padY := gtx.Dp(popupPadX), gtx.Dp(popupPadY)
	full := p.size.Add(image.Pt(2*padX, 2*padY))
	size := image.Pt(min(full.X, maxW+2*padX), min(full.Y, gtx.Dp(popupMaxHeight), e.roomForPopup(gtx)))
	r := place(size)
	if r.Empty() {
		return
	}
	scroll := &e.popups.scroll[kind]
	*scroll = min(max(*scroll, 0), full.Y-r.Dy())
	e.paintPopupBox(gtx, r)
	e.paintProse(gtx, p, r, *scroll)
	e.popupHandler(gtx, kind, r)
}

// roomForPopup is the most a popup can take of the editor's height.
func (e *Editor) roomForPopup(gtx layout.Context) int {
	return e.f.size.Y - 2*gtx.Dp(popupGap)
}
