package godemirror

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

const (
	affordancePadding = unit.Dp(6)
	affordanceRadius  = unit.Dp(4)
	// maxAffordanceLabels bounds the label images kept between frames; past
	// it they are composed again.
	maxAffordanceLabels = 64
)

func defaultExpandLabel(hidden int) string {
	if hidden == 1 {
		return "1 unchanged line"
	}
	return fmt.Sprintf("%d unchanged lines", hidden)
}

// labelKey names a label image by everything it holds.
type labelKey struct {
	text  string
	px    float64
	color color.NRGBA
}

// affordances are the widgets a DiffView puts on its rows, with the state
// that must outlive a rebuild: the clickables behind the default ones, by the
// gap or chunk they act on, and the actions their clicks queued this frame.
type affordances struct {
	expand map[int]*widget.Clickable
	revert map[int]*widget.Clickable
	labels map[labelKey]*rowImage
	queued []func()
}

func (a *affordances) reset() {
	a.expand, a.revert = map[int]*widget.Clickable{}, map[int]*widget.Clickable{}
}

func clickable(m map[int]*widget.Clickable, key int) *widget.Clickable {
	c, ok := m[key]
	if !ok {
		c = new(widget.Clickable)
		m[key] = c
	}
	return c
}

// queue defers action until the frame has been drawn, so a caller's callback
// may set a new diff without pulling the rows out from under the paint.
func (a *affordances) queue(action func()) func() {
	return func() { a.queued = append(a.queued, action) }
}

func (a *affordances) runQueued() {
	queued := a.queued
	a.queued = nil
	for _, action := range queued {
		action()
	}
}

// widgets are the affordances of one side: an expander on each separator
// that can be expanded, and, when the caller reverts, a revert on each
// chunk's first row while the chunk is under the pointer.
func (d *DiffView) widgets(s side, v *View) []LineWidget {
	var out []LineWidget
	for row, g := range s.gaps {
		expand, ok := d.expander(*g)
		if !ok {
			continue
		}
		w := d.opts.ExpandWidget
		if w == nil {
			w = func(g Gap, expand func()) layout.Widget {
				return d.defaultAffordance(v, clickable(d.affordances.expand, g.key), d.expandLabel(g), false, expand)
			}
		}
		out = append(out, LineWidget{Line: row, Placement: AfterText, Widget: w(*g, expand)})
	}
	if d.opts.Revert == nil {
		return out
	}
	for chunk, row := range s.reverts {
		c := d.chunks[chunk-1]
		revert := d.affordances.queue(func() { d.opts.Revert(c) })
		w := d.opts.RevertWidget
		if w == nil {
			w = func(_ Chunk, revert func()) layout.Widget {
				return d.defaultAffordance(v, clickable(d.affordances.revert, chunk), "Revert", true, revert)
			}
		}
		out = append(out, LineWidget{Line: row, Placement: RightEdge, WhileHovered: true, Widget: w(c, revert)})
	}
	return out
}

// expander is what expanding g does: fill it from the modified document when
// the diff has it, or ask the caller. ok is false when neither can.
func (d *DiffView) expander(g Gap) (expand func(), ok bool) {
	if lines := d.src.newLines; lines != nil {
		return d.affordances.queue(func() { d.FillGap(g, lines[g.From-1:g.To]) }), true
	}
	if d.opts.Expand == nil {
		return nil, false
	}
	return d.affordances.queue(func() { d.opts.Expand(g) }), true
}

func (d *DiffView) expandLabel(g Gap) string {
	label := d.opts.ExpandLabel
	if label == nil {
		label = defaultExpandLabel
	}
	return label(g.To - g.From + 1)
}

// defaultAffordance is text drawn in the view's own glyphs, in its gutter's
// color, on the theme's hover color while the pointer is over it; outlined,
// it sits on the background inside a border, to stand out on a tinted row.
func (d *DiffView) defaultAffordance(v *View, c *widget.Clickable, text string, outlined bool, onClick func()) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		for c.Clicked(gtx) {
			onClick()
		}
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			img := d.labelImage(v, text)
			pad := gtx.Dp(affordancePadding)
			size := image.Pt(img.size.X+2*pad, img.size.Y)
			shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(affordanceRadius))
			t := v.Theme
			switch {
			case c.Hovered():
				paint.FillShape(gtx.Ops, over(t.Diff.AffordanceHover, t.Background), shape.Op(gtx.Ops))
			case outlined:
				paint.FillShape(gtx.Ops, t.Border, shape.Op(gtx.Ops))
				inner := clip.UniformRRect(image.Rect(1, 1, size.X-1, size.Y-1), max(gtx.Dp(affordanceRadius)-1, 0))
				paint.FillShape(gtx.Ops, t.Background, inner.Op(gtx.Ops))
			}
			area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			area.Pop()
			off := op.Offset(image.Pt(pad, 0)).Push(gtx.Ops)
			text := clip.Rect(image.Rectangle{Max: img.size}).Push(gtx.Ops)
			img.op.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			text.Pop()
			off.Pop()
			return layout.Dimensions{Size: size}
		})
	}
}

// labelImage is text composed in v's glyphs, kept by everything it holds.
func (d *DiffView) labelImage(v *View, text string) *rowImage {
	k := labelKey{text, v.px, v.Theme.GutterForeground}
	if img, ok := d.affordances.labels[k]; ok {
		return img
	}
	if len(d.affordances.labels) >= maxAffordanceLabels {
		d.affordances.labels = map[labelKey]*rowImage{}
	}
	img := v.compose([]rune(text), nil)
	d.affordances.labels[k] = img
	return img
}
