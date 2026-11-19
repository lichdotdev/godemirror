package godemirror

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
// the document, as CodeMirror's scrollDOM.scrollTop.
func (v *View) ScrollTop() int { return v.scroll.Y }

// SetScrollTop scrolls the view to y pixels from the top of the document,
// as setting CodeMirror's scrollDOM.scrollTop does. The next Layout keeps it
// within the document.
func (v *View) SetScrollTop(y int) { v.scroll.Y = y }

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

// SetScrollTop is View.SetScrollTop of the editor.
func (e *Editor) SetScrollTop(y int) { e.view.SetScrollTop(y) }

// LineBlockAt is where the line pos is on shows, as View.LineBlock and
// CodeMirror's lineBlockAt. pos is a position in the document the last
// Layout showed, which a Dispatch since does not change.
func (e *Editor) LineBlockAt(pos int) (top, bottom int) {
	return e.view.LineBlock(e.shown.LineAt(pos).Number - 1)
}

// LineBlockAtHeight is the line at y pixels from the top of the document,
// as View.LineAtHeight and CodeMirror's lineBlockAtHeight, a line of the
// document the last Layout showed.
func (e *Editor) LineBlockAtHeight(y int) Line { return e.shown.Line(e.view.LineAtHeight(y) + 1) }

// Geometry is View.Geometry of the editor.
func (e *Editor) Geometry() Geometry { return e.view.Geometry() }

// ContentHeight is View.ContentHeight of the editor.
func (e *Editor) ContentHeight() int { return e.view.ContentHeight() }
