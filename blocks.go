package godemirror

import (
	"image"
	"sort"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// BlockWidget is a Gio widget laid out between two lines, across the code
// area, as tall as it lays itself out: a review thread under the line it is
// about, or the box a comment is being typed into. It takes the pointer over
// it; the view does not select or hover there.
//
// Its state is the caller's, held outside the view, so rebuilding the content
// around it (a gap pulled in, a thread resolved) never loses what is half
// typed inside.
type BlockWidget struct {
	// After is the document line (0-based) the block opens under.
	After int
	// Widget is laid out exactly once every frame, on screen or not, so the
	// document's height is known; its height is the one it lays itself out
	// at, and its width the code area's.
	Widget layout.Widget
}

// laidBlock is one block laid out this frame, waiting to be placed.
type laidBlock struct {
	call   op.CallOp
	height int
}

// layoutBlocks lays every block out once, on screen or not, so the height of
// the document is known before anything is placed. A block off screen is
// laid out and then dropped: each widget still gets exactly one layout a
// frame, which is what Gio's widgets expect.
func (v *View) layoutBlocks(gtx layout.Context, f frame) {
	v.blocks = map[int][]laidBlock{}
	v.extra = make([]int, len(v.doc.lines))
	width := max(f.size.X-f.g.gutterW, 0)
	for _, b := range v.content.Blocks {
		if b.After < 0 || b.After >= len(v.doc.lines) {
			continue
		}
		bgtx := gtx
		bgtx.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, 1<<24)}
		macro := op.Record(gtx.Ops)
		dims := b.Widget(bgtx)
		laid := laidBlock{call: macro.Stop(), height: dims.Size.Y}
		v.blocks[b.After] = append(v.blocks[b.After], laid)
		v.extra[b.After] += laid.height
	}
}

// visibleBlock is a block on screen, at y.
type visibleBlock struct {
	laidBlock
	y int
}

// visible is what of the document is on screen: its rows and its blocks.
func (v *View) visible(f frame) (rows []visibleRow, blocks []visibleBlock) {
	first := sort.Search(len(v.tops)-1, func(i int) bool { return v.tops[i+1] > v.scroll.Y })
	for line := first; line < len(v.tops)-1 && v.tops[line]-v.scroll.Y < f.size.Y; line++ {
		y := v.tops[line] - v.scroll.Y
		for segment := range v.starts[line+1] - v.starts[line] {
			if y+f.ch > 0 && y < f.size.Y {
				rows = append(rows, visibleRow{line: line, segment: segment, y: y})
			}
			y += f.ch
		}
		for _, b := range v.blocks[line] {
			if y+b.height > 0 && y < f.size.Y {
				blocks = append(blocks, visibleBlock{b, y})
			}
			y += b.height
		}
	}
	return rows, blocks
}

func (v *View) paintBlocks(gtx layout.Context, blocks []visibleBlock, code image.Rectangle) {
	defer clip.Rect(code).Push(gtx.Ops).Pop()
	for _, b := range blocks {
		at := image.Pt(code.Min.X, b.y)
		v.widgetAreas = append(v.widgetAreas, image.Rectangle{Min: at, Max: image.Pt(code.Max.X, b.y+b.height)})
		t := op.Offset(at).Push(gtx.Ops)
		b.call.Add(gtx.Ops)
		t.Pop()
	}
}
