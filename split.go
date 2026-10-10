package godemirror

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/glyphs"
)

// Split shows two documents side by side whose line i pairs with line i of
// the other: the two sides of a diff. Each pair of lines starts on the same
// row: unwrapped that holds by itself, and when the sides wrap, the side that
// wraps into fewer rows is padded to the other's. A block under a line on one
// side leaves as much room under its pair on the other. They scroll down
// together and sideways each on its own.
//
// Content, Theme and selection are each side's own, set on Left and Right;
// Wrap is the split's, and Layout sets it on both sides.
type Split struct {
	// Left and Right are the two sides. Each is a View, but it is laid out
	// by the Split's Layout, never its own.
	Left, Right *View
	// Wrap breaks both sides' lines instead of scrolling them sideways.
	Wrap bool
}

// NewSplit builds a split view whose two sides draw family at size, from one
// set of loaded fonts. It fails as New does.
func NewSplit(family string, size unit.Sp) (*Split, error) {
	r, err := glyphs.New(family)
	if err != nil {
		return nil, err
	}
	return &Split{Left: newView(r, size), Right: newView(r, size)}, nil
}

// SetTheme gives both sides t. The divider between them takes the left
// side's Theme.Border.
func (s *Split) SetTheme(t Theme) {
	s.Left.Theme, s.Right.Theme = t, t
}

// Layout draws both sides into the whole of gtx.Constraints.Max, halved, with
// a 1dp Border between them. Its error means what View.Layout's does,
// for either side.
func (s *Split) Layout(gtx layout.Context) (layout.Dimensions, error) {
	size := gtx.Constraints.Max
	divider := max(gtx.Dp(1), 1)
	leftW := (size.X - divider) / 2
	rightX := leftW + divider
	lgtx, rgtx := gtx, gtx
	lgtx.Constraints = layout.Exact(image.Pt(leftW, size.Y))
	rgtx.Constraints = layout.Exact(image.Pt(size.X-rightX, size.Y))

	s.Left.Wrap, s.Right.Wrap = s.Wrap, s.Wrap
	lf, err := s.Left.prepare(lgtx)
	if err != nil {
		return layout.Dimensions{}, err
	}
	rf, err := s.Right.prepare(rgtx)
	if err != nil {
		return layout.Dimensions{}, err
	}
	pad := pairedRows(s.Left.wraps, s.Right.wraps)
	s.Left.starts = rowStarts(s.Left.wraps, pad)
	s.Right.starts = rowStarts(s.Right.wraps, pad)
	// A block on one side leaves the same room on the other, so the lines
	// under it stay level.
	extra := pairedHeights(s.Left.extra, s.Right.extra)
	s.Left.tops = lineTops(s.Left.starts, lf.ch, extra)
	s.Right.tops = lineTops(s.Right.starts, rf.ch, extra)
	if s.Left.scrollToTarget(lf) {
		s.Right.scroll.Y = s.Left.scroll.Y
	}
	if s.Right.scrollToTarget(rf) {
		s.Left.scroll.Y = s.Right.scroll.Y
	}
	s.input(lgtx, rgtx, lf, rf)

	s.Left.paint(lgtx, lf)
	paint.FillShape(gtx.Ops, s.Left.Theme.Border, clip.Rect(image.Rect(leftW, 0, rightX, size.Y)).Op())
	defer op.Offset(image.Pt(rightX, 0)).Push(gtx.Ops).Pop()
	s.Right.paint(rgtx, rf)
	return layout.Dimensions{Size: size}, nil
}

// input lets each side take its own pointer and keys, then carries whichever
// side moved down or up over to the other. A selection lives on one side at a
// time: starting one clears the other's. The hover group is shared.
func (s *Split) input(lgtx, rgtx layout.Context, lf, rf frame) {
	leftY, rightY := s.Left.scroll.Y, s.Right.scroll.Y
	if s.Left.handlePointer(lgtx, lf) {
		s.Right.ClearSelection()
	}
	if s.Right.handlePointer(rgtx, rf) {
		s.Left.ClearSelection()
	}
	s.Left.handleKeys(lgtx)
	s.Right.handleKeys(rgtx)
	// A hover group is one change seen from both sides: lit from either, it
	// lights on both.
	hot := s.Left.hot
	if s.Right.hover >= 0 {
		hot = s.Right.hot
	}
	s.Left.hot, s.Right.hot = hot, hot
	switch {
	case s.Left.scroll.Y != leftY:
		s.Right.scroll.Y = s.Left.scroll.Y
	case s.Right.scroll.Y != rightY:
		s.Left.scroll.Y = s.Right.scroll.Y
	}
}

// pairedRows is, per line, the rows the taller of the pair wraps into.
func pairedRows(left, right [][]int) []int {
	rows := make([]int, max(len(left), len(right)))
	for i := range rows {
		if i < len(left) {
			rows[i] = len(left[i])
		}
		if i < len(right) {
			rows[i] = max(rows[i], len(right[i]))
		}
	}
	return rows
}

// pairedHeights is, per line, the taller of the two sides' blocks.
func pairedHeights(left, right []int) []int {
	out := make([]int, max(len(left), len(right)))
	for i := range out {
		if i < len(left) {
			out[i] = left[i]
		}
		if i < len(right) {
			out[i] = max(out[i], right[i])
		}
	}
	return out
}
