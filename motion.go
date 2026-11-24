package godemirror

import (
	"slices"
	"unicode/utf8"
)

// The motions in this file port what @codemirror/view 6.43.15's cursor.ts and
// @codemirror/commands 6.11.1 do with a laid-out editor, on the view's grid
// of cells: positions on screen are rows of cells, wrapped or not, instead of
// the browser's boxes. They read the frame the editor last laid out, so they
// run during its Layout.

// rowOf is the wrapped segment of line that cell shows on, a cell on the
// boundary of two going to the row above when assoc is negative.
func (e *Editor) rowOf(line, cell, assoc int) int {
	wraps := e.view.wraps[line]
	seg, _ := slices.BinarySearch(wraps, cell+1)
	seg--
	if seg > 0 && wraps[seg] == cell && assoc < 0 {
		seg--
	}
	return max(seg, 0)
}

// coordsAt is where pos shows, as CodeMirror's coordsAtPos with side assoc:
// x in pixels from the start of the text, and the top of its row in pixels
// from the top of the document.
func (e *Editor) coordsAt(pos, assoc int) (x, y int) { return e.coordsOf(e.cellAt(pos), assoc) }

// coordsOf is coordsAt of p, a place between the view's cells.
func (e *Editor) coordsOf(p pos, assoc int) (x, y int) {
	v, f := e.view, e.f
	line, cell := p.line, p.col
	seg := e.rowOf(line, cell, assoc)
	return (cell - v.wraps[line][seg] + v.hang(line, seg)) * f.cw, v.tops[line] + seg*f.ch
}

// posAtCoords is the position nearest the point x, y (as coordsAt gives
// them) and the side it was found on, as CodeMirror's posAtCoords: above the
// document its start, below its end, past the end of a row that row's end.
func (e *Editor) posAtCoords(x, y int) (offset, assoc int) {
	v, f := e.view, e.f
	line, seg, ok := locate(v.tops, v.starts, f.ch, y)
	switch {
	case !ok && y < 0:
		return 0, 1
	case !ok:
		return len(e.state.doc), -1
	case seg < 0:
		return e.state.Line(line + 1).To, -1
	}
	from, to := v.segmentSpan(line, seg)
	want := float64(from) + float64(x)/float64(f.cw)
	if hang := v.hang(line, seg); hang > 0 {
		// The indent is not text: a point in it is before the row's first
		// cell, on its own row, not past the end of the row above.
		want = max(want-float64(hang), float64(from))
	}
	cell, best := to, float64(to)-want
	if best < 0 {
		best = -best
	}
	for _, c := range v.doc.cellOf[line] {
		if c < from || c > to {
			continue
		}
		if d := abs64(float64(c) - want); d < best {
			cell, best = c, d
		}
	}
	assoc = 1
	if float64(cell) > want || cell == to && seg+1 < len(v.wraps[line]) {
		assoc = -1
	}
	return e.offsetAt(pos{line, cell}), assoc
}

func abs64(v float64) float64 { return max(v, -v) }

// moveVertically is @codemirror/view's moveVertically: start's head moved a
// row up or down, or distance pixels, keeping to its goal column.
func (e *Editor) moveVertically(start SelectionRange, forward bool, distance int) SelectionRange {
	head, dir := start.Head(), -1
	if forward {
		dir = 1
	}
	if forward && head == len(e.state.doc) || !forward && head == 0 {
		return cursorAssoc(head, start.Assoc())
	}
	side := start.Assoc()
	if side == 0 {
		side = -1
		if start.Empty() && forward || !start.Empty() && head == start.from {
			side = 1
		}
	}
	x, top := e.coordsAt(head, side)
	goal := x
	if start.goal != noGoal {
		goal = start.goal * e.f.cw
	}
	startY := top
	if forward {
		startY = top + e.f.ch
	}
	half := e.f.ch / 2
	if distance == 0 {
		distance = half
	}
	docHeight := e.view.tops[len(e.view.tops)-1]
	for scan := 0; ; scan += half {
		y := startY + (distance+scan)*dir
		pos, assoc := e.posAtCoords(goal, y)
		if forward && y > docHeight || !forward && y < 0 {
			return cursorAssoc(pos, assoc)
		}
		_, posTop := e.coordsAt(pos, assoc)
		mid := posTop + e.f.ch/2
		if forward && mid > startY || !forward && mid < startY {
			r := cursorAssoc(pos, assoc)
			r.goal = goal / e.f.cw
			return r
		}
	}
}

// moveToLineBoundary is @codemirror/view's moveToLineBoundary: to the end
// or the start of the row start's head is on when includeWrap is set and the
// line wraps, else of its line.
func (e *Editor) moveToLineBoundary(start SelectionRange, forward, includeWrap bool) SelectionRange {
	head := start.Head()
	l := e.state.LineAt(head)
	if includeWrap && e.view.Wrap {
		at := head
		if start.Assoc() < 0 && head > l.From {
			at = prevRuneStart(e.state.doc, head)
		}
		p := e.cellAt(at)
		line, cell := p.line, p.col
		if wraps := e.view.wraps[line]; len(wraps) > 1 {
			from, to := e.view.segmentSpan(line, e.rowOf(line, cell, start.Assoc()))
			if forward {
				return cursorAssoc(e.offsetAt(pos{line, to}), -1)
			}
			return cursorAssoc(e.offsetAt(pos{line, from}), 1)
		}
	}
	if forward {
		return cursorAssoc(l.To, 1)
	}
	return cursorAssoc(l.From, -1)
}

func prevRuneStart(s string, pos int) int {
	_, n := utf8.DecodeLastRuneInString(s[:pos])
	return pos - n
}

// moveByLineBoundary is @codemirror/commands' moveByLineBoundary: Home goes
// to the row's start, then the line's, then to the end of the line's
// indentation and back; End to the row's end, then the line's.
func (e *Editor) moveByLineBoundary(start SelectionRange, forward bool) SelectionRange {
	l := e.state.LineAt(start.Head())
	moved := e.moveToLineBoundary(start, forward, true)
	edge := l.From
	if forward {
		edge = l.To
	}
	if moved.Head() == start.Head() && moved.Head() != edge {
		moved = e.moveToLineBoundary(start, forward, false)
	}
	if !forward && moved.Head() == l.From && len(l.Text) > 0 {
		space := len(leadingSpace(utf16Prefix(l.Text, 100)))
		if space > 0 && start.Head() != l.From+space {
			moved = Cursor(l.From + space)
		}
	}
	return moved
}

// pageHeight is how far a page moves, as @codemirror/commands' pageInfo for
// an editor that scrolls itself: the view's height less 5 pixels.
func (e *Editor) pageHeight() int { return max(e.f.ch, e.f.size.Y-5) }

func (e *Editor) cursorByLine(forward bool) (TransactionSpec, bool) {
	return e.state.moveSel(func(r SelectionRange) SelectionRange {
		if !r.Empty() {
			return rangeEnd(r, forward)
		}
		moved := e.moveVertically(r, forward, 0)
		if moved.Head() != r.Head() {
			return moved
		}
		return e.moveToLineBoundary(r, forward, true)
	})
}

func (e *Editor) selectByLine(forward bool) (TransactionSpec, bool) {
	return e.state.extendSel(func(r SelectionRange) SelectionRange { return e.moveVertically(r, forward, 0) })
}

// cursorByPage moves every cursor a page and, when the main one was on
// screen, scrolls by as much, so it stays where it was on screen.
func (e *Editor) cursorByPage(forward bool) (TransactionSpec, bool) {
	sel := e.state.updateSel(func(r SelectionRange) SelectionRange {
		if r.Empty() {
			return e.moveVertically(r, forward, e.pageHeight())
		}
		return rangeEnd(r, forward)
	})
	if sel.eq(e.state.sel, false) {
		return TransactionSpec{}, false
	}
	main := e.state.sel.Main()
	_, top := e.coordsAt(main.Head(), main.Assoc())
	if onScreen := top - e.view.scroll.Y; onScreen > 0 && onScreen+e.f.ch < e.f.size.Y {
		e.keepOnScreen = &onScreen
	}
	return setSel(sel), true
}

func (e *Editor) selectByPage(forward bool) (TransactionSpec, bool) {
	return e.state.extendSel(func(r SelectionRange) SelectionRange {
		return e.moveVertically(r, forward, e.pageHeight())
	})
}

func (e *Editor) cursorLineBoundary(forward bool) (TransactionSpec, bool) {
	return e.state.moveSel(func(r SelectionRange) SelectionRange { return e.moveByLineBoundary(r, forward) })
}

func (e *Editor) selectLineBoundary(forward bool) (TransactionSpec, bool) {
	return e.state.extendSel(func(r SelectionRange) SelectionRange { return e.moveByLineBoundary(r, forward) })
}
