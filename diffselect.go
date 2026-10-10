package godemirror

import "gioui.org/layout"

// LineRef names a line of one file of a diff, 1-based.
type LineRef struct {
	Side Side
	Line int
}

// DiffBlock is a BlockWidget anchored to a line of a file instead of a row
// of the layout: a review thread under the line it is about, wherever
// unified or side by side puts that line. Side by side, it opens on its
// line's side and leaves the same room on the other.
type DiffBlock struct {
	At     LineRef
	Widget layout.Widget
}

// DiffRange is a span of lines in each file, 1-based and inclusive; a side
// with no line in the span has From and To 0.
type DiffRange struct {
	OldFrom, OldTo, NewFrom, NewTo int
}

// SetBlocks lays blocks out under their lines, replacing those set before,
// from the next Layout once highlighting allows (as UpdateContent does). A
// block whose line is in a gap, or not in the diff, is not shown until the
// line is. Its Widget is laid out as a BlockWidget's is: once a frame.
func (d *DiffView) SetBlocks(blocks []DiffBlock) {
	d.blocks = blocks
	d.update()
}

// blocksOf places the blocks on the rows of s that show their lines, for
// blocks of the sides it shows.
func (d *DiffView) blocksOf(s side, shows func(Side) bool) []BlockWidget {
	var out []BlockWidget
	for _, b := range d.blocks {
		if !shows(b.At.Side) {
			continue
		}
		for i, r := range s.rows {
			if lineOn(r, b.At.Side) == b.At.Line {
				out = append(out, BlockWidget{After: i, Widget: b.Widget})
				break
			}
		}
	}
	return out
}

// lineOn is r's number in the file of side, 0 when it has none there.
func lineOn(r diffRow, side Side) int {
	if side == OldSide {
		return r.oldLine
	}
	return r.newLine
}

// SelectedRange is the span of each file's lines the selection covers, in
// whichever layout shows it, or ok false when nothing is selected. Side by
// side, a selection lives on one side, which shows one file, so the other's
// span is 0.
func (d *DiffView) SelectedRange() (r DiffRange, ok bool) {
	rows, v, ok := d.selectedRows()
	if !ok {
		return DiffRange{}, false
	}
	for _, row := range rows {
		if v != d.split.Right {
			r.OldFrom, r.OldTo = widen(r.OldFrom, r.OldTo, row.oldLine)
		}
		if v != d.split.Left {
			r.NewFrom, r.NewTo = widen(r.NewFrom, r.NewTo, row.newLine)
		}
	}
	return r, true
}

func widen(from, to, line int) (int, int) {
	if line == 0 {
		return from, to
	}
	if from == 0 {
		return line, line
	}
	return min(from, line), max(to, line)
}

// SelectedChanges are the changed lines the selection covers, in diff order
// and as Chunk.Lines names them, ready for DiffOptions.Revert's caller to
// revert just those; nil when nothing changed is selected.
func (d *DiffView) SelectedChanges() []ChangedLine {
	rows, _, _ := d.selectedRows()
	var out []ChangedLine
	for _, r := range rows {
		if r.kind == rowAdded || r.kind == rowDeleted {
			out = append(out, changedLine(r))
		}
	}
	return out
}

// selectedRows are the rows the selection covers and v the view that holds
// it, read against the rows that view has on screen.
func (d *DiffView) selectedRows() (rows []diffRow, v *View, ok bool) {
	views := []*View{d.unified}
	if d.Split {
		views = []*View{d.split.Left, d.split.Right}
	}
	for _, v := range views {
		from, to, ok := v.SelectedLines()
		if !ok {
			continue
		}
		rows := d.onScreen[v].rows(v)
		return rows[min(from, len(rows)):min(to+1, len(rows))], v, true
	}
	return nil, nil, false
}

// viewRows are the rows of the content a view shows and of the content it
// was last given, which UpdateContent swaps in only once highlighted.
type viewRows struct{ shown, next []diffRow }

func (r *viewRows) rows(v *View) []diffRow {
	if v.pending == nil || v.pending.content == nil {
		r.shown = r.next
	}
	return r.shown
}
