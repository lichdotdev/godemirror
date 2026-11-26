package godemirror

import "image"

// The geometry below answers, for a View or an Editor laid out on its own,
// what @codemirror/view 6.43.15's EditorView answers with
// scrollDOM.scrollTop, lineBlockAt, lineBlockAtHeight and contentHeight:
// heights in pixels from the top of the document, as the last Layout placed
// it. A Split or a DiffView places its views itself.

// Geometry is where a view's last Layout put its gutter and its text across,
// in pixels from the view's left edge: what a caller needs to draw beside
// the gutter or past a line's text, which CodeMirror leaves to the DOM.
type Geometry struct {
	// GutterWidth is the gutter's width; the code area starts where it ends.
	GutterWidth int
	// TextX is where a line's first cell starts while the view is not
	// scrolled sideways.
	TextX int
	// CellWidth is one cell's width; a wide character takes two.
	CellWidth int
}

// Geometry is the gutter and the cells of the last Layout, all 0 before
// any. A Split's or a DiffView's views measure their own.
func (v *View) Geometry() Geometry { return v.laidOut }

// ScrollTop is how far the view is scrolled down, in pixels from the top of
// the document, as CodeMirror's scrollDOM.scrollTop: where the last Layout
// drew it, partway through a glide.
func (v *View) ScrollTop() int { return v.scroll.Y }

// ScrollTarget is where the view lands once its glide is over, ScrollTop
// when it is not gliding. A target SetScrollTop set since the last Layout
// is not yet kept within the document.
func (v *View) ScrollTarget() int { return v.heading() }

// SetScrollTop scrolls the view to y pixels from the top of the document,
// as setting CodeMirror's scrollDOM.scrollTop does, but glides there over
// the next frames, eased out, unless the jump is a few rows or ReduceMotion
// is set. The next Layout keeps it within the document. A wheel during the
// glide stops it where the view is.
func (v *View) SetScrollTop(y int) { v.glideTo(y) }

// SnapScrollTop is SetScrollTop without the glide: the next Layout draws the
// view at y, within the document, and stops any glide.
func (v *View) SnapScrollTop(y int) { v.scroll.Y, v.glide = y, glide{} }

// LineBlock is where line (0-based) shows, as CodeMirror's lineBlockAt: the
// top and bottom of its rows, wrapped ones included, in pixels from the top
// of the document, as the last Layout placed them; blocks under the line
// are not part of it. For content set since the last Layout, it is an
// estimate, as CodeMirror's height map estimates lines it has not measured:
// a row a line and no blocks. Before any Layout, both are 0. line must be in
// [0, LineCount()).
func (v *View) LineBlock(line int) (top, bottom int) {
	tops, starts := v.placed()
	top = tops[line]
	return top, top + (starts[line+1]-starts[line])*v.r.CellH()
}

// LineAtHeight is the line (0-based) at y pixels from the top of the
// document, as CodeMirror's lineBlockAtHeight: the first line above the
// document, the last below it, and the line a block sits under over the
// block. It estimates as LineBlock does.
func (v *View) LineAtHeight(y int) int {
	tops, starts := v.placed()
	line, _, ok := locate(tops, starts, v.r.CellH(), y)
	switch {
	case ok:
		return line
	case y < 0:
		return 0
	}
	return len(tops) - 2
}

// ContentHeight is the document's height in pixels as the last Layout
// placed it, blocks included, as CodeMirror's contentHeight. It estimates
// as LineBlock does.
func (v *View) ContentHeight() int {
	tops, _ := v.placed()
	return tops[len(tops)-1]
}

// placed are the tops and starts of the lines on screen: the last
// Layout's, or, for a document set since, a row a line at the size last laid
// out, all 0 before any.
func (v *View) placed() (tops, starts []int) {
	if v.laid == v.doc {
		return v.tops, v.starts
	}
	n, ch := len(v.doc.lines), v.r.CellH()
	tops, starts = make([]int, n+1), make([]int, n+1)
	for i := range n + 1 {
		tops[i], starts[i] = i*ch, i
	}
	return tops, starts
}

// ScrollTop is View.ScrollTop of the editor.
func (e *Editor) ScrollTop() int { return e.view.ScrollTop() }

// ScrollTarget is View.ScrollTarget of the editor.
func (e *Editor) ScrollTarget() int { return e.view.ScrollTarget() }

// SetScrollTop is View.SetScrollTop of the editor.
func (e *Editor) SetScrollTop(y int) { e.view.SetScrollTop(y) }

// SnapScrollTop is View.SnapScrollTop of the editor.
func (e *Editor) SnapScrollTop(y int) { e.view.SnapScrollTop(y) }

// LineBlockAt is where the line pos is on shows, as View.LineBlock and
// CodeMirror's lineBlockAt. pos is a position in the document the last
// Layout showed, which a Dispatch since does not change.
func (e *Editor) LineBlockAt(pos int) (top, bottom int) {
	return e.view.LineBlock(e.shown.LineAt(pos).Number - 1)
}

// CoordsAtPos is where pos shows, as CodeMirror's coordsAtPos: a rectangle
// of no width from the top to the bottom of its row, in pixels from the
// editor's top left, as the last Layout placed it with its gutter and
// scrolling. A negative side puts pos with the character before it, any
// other with the one after, which tells where a line breaks under Wrap: at
// the end of the row above or the start of the next. pos is a position in
// the document the last Layout showed, which a Dispatch since does not
// change. It reports false, where CodeMirror returns null, before the first
// Layout, for a state set since, and for pos outside the document or inside
// a character. Scrolled away, the rectangle lies outside the editor.
func (e *Editor) CoordsAtPos(pos, side int) (image.Rectangle, bool) {
	v, f, s := e.view, e.f, e.shown
	if v.laid != v.doc || pos < 0 || pos > len(s.doc) || s.checkBoundary(pos) != nil {
		return image.Rectangle{}, false
	}
	x, top := e.coordsOf(e.cellIn(s, pos), side)
	x += f.g.textX - v.scrollX()
	y := top - v.scroll.Y
	return image.Rect(x, y, x, y+f.ch), true
}

// LineBlockAtHeight is the line at y pixels from the top of the document,
// as View.LineAtHeight and CodeMirror's lineBlockAtHeight, a line of the
// document the last Layout showed.
func (e *Editor) LineBlockAtHeight(y int) Line { return e.shown.Line(e.view.LineAtHeight(y) + 1) }

// Geometry is View.Geometry of the editor.
func (e *Editor) Geometry() Geometry { return e.view.Geometry() }

// ContentHeight is View.ContentHeight of the editor.
func (e *Editor) ContentHeight() int { return e.view.ContentHeight() }
