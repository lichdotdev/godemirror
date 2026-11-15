package godemirror

// The geometry below answers, for a View or an Editor laid out on its own,
// what @codemirror/view 6.43.15's EditorView answers with
// scrollDOM.scrollTop, lineBlockAt, lineBlockAtHeight and contentHeight:
// heights in pixels from the top of the document, as the last Layout placed
// it. A Split or a DiffView places its views itself.

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
// are not part of it. line must be in [0, LineCount()); before a first
// Layout, both are 0.
func (v *View) LineBlock(line int) (top, bottom int) {
	if len(v.tops) < 2 || len(v.starts) < 2 {
		return 0, 0
	}
	top = v.tops[line]
	return top, top + (v.starts[line+1]-v.starts[line])*v.r.CellH()
}

// LineAtHeight is the line (0-based) at y pixels from the top of the
// document, as CodeMirror's lineBlockAtHeight: the first line above the
// document, the last below it, and the line a block sits under over the
// block.
func (v *View) LineAtHeight(y int) int {
	if len(v.tops) < 2 {
		return 0
	}
	line, _, ok := locate(v.tops, v.starts, v.r.CellH(), y)
	switch {
	case ok:
		return line
	case y < 0:
		return 0
	}
	return len(v.tops) - 2
}

// ContentHeight is the document's height in pixels as the last Layout
// placed it, blocks included, as CodeMirror's contentHeight.
func (v *View) ContentHeight() int {
	if len(v.tops) == 0 {
		return 0
	}
	return v.tops[len(v.tops)-1]
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

// ContentHeight is View.ContentHeight of the editor.
func (e *Editor) ContentHeight() int { return e.view.ContentHeight() }
