package godemirror

import "gioui.org/layout"

// Height is how tall the diff's rows are when laid out width pixels wide, in
// the mode it is in now (Split, Wrap) at the size and scale of gtx, so a
// caller can give a DiffView exactly the room it needs, a file's diff in a
// list of them, say, without laying it out. With Wrap, a line takes as many
// rows as it wraps into at that width, and a side-by-side row is as tall as
// the taller of its two sides.
//
// It counts the diff as set, each gap filled so far included, but not the
// BlockWidgets, whose height only their own layout knows: the caller adds
// what it knows of them. Without Wrap it is the row count times the row
// height; with it, every row is measured, a cost to keep off the frame path
// of a very long diff by calling it only when the diff, the width or Wrap
// change. Its error means what View.Layout's does for the font size.
func (d *DiffView) Height(gtx layout.Context, width int) (int, error) {
	r := d.unified.r
	if err := r.SetSize(float64(gtx.Sp(d.unified.size))); err != nil {
		return 0, err
	}
	cw, ch := r.CellW(), r.CellH()
	if !d.Split {
		return rowCount(d.unifiedSide, d.Wrap, geometryOf(widestLabel(d.unifiedSide.labels), width, cw).cols) * ch, nil
	}
	leftW, rightW := splitWidths(width, splitDivider(gtx))
	left, right := d.leftSide, d.rightSide
	if !d.Wrap {
		return len(left.rows) * ch, nil
	}
	lcols := geometryOf(widestLabel(left.labels), leftW, cw).cols
	rcols := geometryOf(widestLabel(right.labels), rightW, cw).cols
	rows := 0
	for i := range left.rows {
		rows += max(rowsOf(left.rows[i], lcols), rowsOf(right.rows[i], rcols))
	}
	return rows * ch, nil
}

// rowCount is how many visual rows a side's lines take, wrapped at cols
// cells or not.
func rowCount(s side, wrap bool, cols int) int {
	if !wrap {
		return len(s.rows)
	}
	rows := 0
	for _, r := range s.rows {
		rows += rowsOf(r, cols)
	}
	return rows
}

// rowsOf is how many visual rows one diff row wraps into at cols cells.
func rowsOf(r diffRow, cols int) int {
	d := newDocument()
	d.add(r.text, nil)
	return len(wrapLine(d.lines[0], cols))
}
