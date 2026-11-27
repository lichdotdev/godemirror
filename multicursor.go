package godemirror

import (
	"image"
	"math"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

// addCursorVertically is @codemirror/commands 6.11.1's function of the same
// name: a cursor added on the line above or below each range's head, at its
// column, made the main range.
func (e *Editor) addCursorVertically(forward bool) (TransactionSpec, bool) {
	s := e.state
	ranges := s.sel.Ranges()
	for _, r := range s.sel.ranges {
		line := s.LineAt(r.Head())
		if forward && line.To >= len(s.doc) || !forward && line.From <= 0 {
			continue
		}
		for cur := r; ; {
			next := e.moveVertically(cur, forward, 0)
			if next.Head() < line.From || next.Head() > line.To {
				if !hasHead(ranges, next.Head()) {
					ranges = append(ranges, next)
				}
				break
			}
			if next.Head() == cur.Head() {
				break
			}
			cur = next
		}
	}
	if len(ranges) == len(s.sel.ranges) {
		return TransactionSpec{}, false
	}
	return setSel(NewSelection(ranges, len(ranges)-1)), true
}

func hasHead(ranges []SelectionRange, head int) bool {
	for _, r := range ranges {
		if r.Head() == head {
			return true
		}
	}
	return false
}

// simplifySelection is @codemirror/commands' simplifySelection: several
// ranges become the main one, a range its head.
func (s EditorState) simplifySelection() (TransactionSpec, bool) {
	main := s.sel.Main()
	switch {
	case len(s.sel.ranges) > 1:
		return setSel(NewSelection([]SelectionRange{main}, 0)), true
	case !main.Empty():
		return setSel(NewSelection([]SelectionRange{Cursor(main.Head())}, 0)), true
	}
	return TransactionSpec{}, false
}

// rectPos is a corner of a rectangular selection, as @codemirror/view's
// rectangular-selection.ts keeps it: a line (from 1), a column, and the
// offset into the line.
type rectPos struct{ line, col, off int }

// rectangleFor is @codemirror/view 6.43.15's rectangleFor: a range per line
// between a and b, over the columns between theirs, a cursor at the end of a
// line too short to reach them. CodeMirror stops counting columns past the
// 2,000th character of a line and falls back on offsets there; godemirror
// always counts columns.
func (s EditorState) rectangleFor(a, b rectPos) []SelectionRange {
	startCol, endCol := min(a.col, b.col), max(a.col, b.col)
	var ranges []SelectionRange
	for i := min(a.line, b.line); i <= max(a.line, b.line); i++ {
		line := s.Line(i)
		start, reached := findColumn(line.Text, startCol, tabSize)
		if !reached {
			ranges = append(ranges, Cursor(line.To))
			continue
		}
		end, _ := findColumn(line.Text, endCol, tabSize)
		ranges = append(ranges, Range(line.From+start, line.From+end))
	}
	return ranges
}

// rectPosAt is the corner a pointer at at makes: past a line's end, its
// column is counted from the pointer's place, as if the line went on.
func (e *Editor) rectPosAt(f frame, at image.Point) rectPos {
	pos, _ := e.posAndSide(f, at)
	line := e.state.LineAt(pos)
	off := pos - line.From
	col := countColumn(line.Text, tabSize, off)
	if off == len(line.Text) {
		x := at.X - f.g.textX + e.view.scrollX()
		col = int(math.Round(math.Abs(float64(x) / float64(f.cw))))
	}
	return rectPos{line.Number, col, off}
}

// rectangle is a rectangular selection held by the pointer, as
// rectangularSelection's mouse selection style: from start, where the press
// was, over startSel, the selection before it, to which a press with Ctrl
// adds.
type rectangle struct {
	start    rectPos
	startSel EditorSelection
	adds     bool
}

func (r *rectangle) get(s EditorState, cur rectPos) EditorSelection {
	ranges := s.rectangleFor(r.start, cur)
	if len(ranges) == 0 {
		return r.startSel
	}
	if r.adds {
		ranges = append(ranges, r.startSel.ranges...)
	}
	return NewSelection(ranges, 0)
}

// mapThrough keeps the press on its line through changes made while it is
// held, as the style's update does.
func (r *rectangle) mapThrough(before, after EditorState, c ChangeSet) {
	line := after.LineAt(c.MapPos(before.Line(r.start.line).From, -1))
	r.start = rectPos{line.Number, r.start.col, min(r.start.off, len(line.Text))}
	r.startSel = r.startSel.Map(c)
}

// trackCrosshair turns the pointer into a crosshair while Alt is held, as
// crosshairCursor does, from what the last key or pointer event said of it.
func (e *Editor) trackCrosshair() {
	if mods := e.view.mods; mods != e.pointerMods {
		e.pointerMods = mods
		e.altDown = mods.Contain(key.ModAlt)
		e.shortcutDown = mods.Contain(key.ModShortcut)
	}
	e.view.cursor = pointer.CursorText
	if e.CrosshairCursor && e.altDown {
		e.view.cursor = pointer.CursorCrosshair
	}
}

// onAltKey follows Alt through the keys the editor takes, as crosshairCursor
// does through key down and key up.
func (e *Editor) onAltKey(ev key.Event) {
	switch {
	case ev.State == key.Press:
		e.altDown = ev.Name == key.NameAlt || ev.Modifiers.Contain(key.ModAlt)
	case ev.Name == key.NameAlt || !ev.Modifiers.Contain(key.ModAlt):
		e.altDown = false
	}
}
