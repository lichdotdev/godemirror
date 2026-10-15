package main

import (
	"image"
	"image/color"
	"slices"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/godemirror"
)

// comment is a review comment under a line of the diff, a DiffBlock: first
// the box it is typed into, then, posted, the comment itself.
type comment struct {
	at     godemirror.LineRef
	editor widget.Editor
	// text is what was posted; "" while it is still being written.
	text   string
	focus  bool
	cancel widget.Clickable
	submit widget.Clickable
	remove widget.Clickable
}

const (
	commentText    = unit.Sp(13)
	commentLines   = 3
	slotPaddingX   = unit.Dp(8)
	slotPaddingY   = unit.Dp(6)
	cardPadding    = unit.Dp(10)
	cardRadius     = unit.Dp(6)
	buttonPadding  = unit.Dp(6)
	buttonRadius   = unit.Dp(4)
	buttonGap      = unit.Dp(4)
	buttonTextSize = unit.Sp(11)
)

// openComment opens a comment box under the last selected line: of the new
// file when the selection has one, of the old file otherwise.
func (d *demo) openComment() {
	r, ok := d.diff.SelectedRange()
	if !ok {
		return
	}
	at := godemirror.LineRef{Side: godemirror.NewSide, Line: r.NewTo}
	if r.NewTo == 0 {
		at = godemirror.LineRef{Side: godemirror.OldSide, Line: r.OldTo}
	}
	d.comments = append(d.comments, &comment{at: at, focus: true})
	d.showComments()
}

func (d *demo) dropComment(c *comment) {
	d.comments = slices.DeleteFunc(d.comments, func(o *comment) bool { return o == c })
	d.showComments()
}

func (d *demo) showComments() {
	blocks := make([]godemirror.DiffBlock, len(d.comments))
	for i, c := range d.comments {
		blocks[i] = godemirror.DiffBlock{At: c.at, Widget: d.commentWidget(c)}
	}
	d.diff.SetBlocks(blocks)
}

func (d *demo) commentWidget(c *comment) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		inset := layout.Inset{Top: slotPaddingY, Bottom: slotPaddingY, Left: slotPaddingX, Right: slotPaddingX}
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return d.card(gtx, func(gtx layout.Context) layout.Dimensions {
				if c.text == "" {
					return d.composer(gtx, c)
				}
				return d.posted(gtx, c)
			})
		})
	}
}

func (d *demo) composer(gtx layout.Context, c *comment) layout.Dimensions {
	if c.focus {
		gtx.Execute(key.FocusCmd{Tag: &c.editor})
		c.focus = false
	}
	for c.cancel.Clicked(gtx) {
		d.dropComment(c)
	}
	for c.submit.Clicked(gtx) {
		if c.editor.Text() != "" {
			c.text = c.editor.Text()
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			e := material.Editor(d.th, &c.editor, "Leave a comment")
			e.TextSize, e.Color, e.HintColor = commentText, d.foreground(), d.theme.GutterForeground
			gtx.Constraints.Min.Y = gtx.Sp(commentText) * commentLines * 3 / 2
			return e.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: cardPadding}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
				layout.Rigid(d.button(&c.cancel, "Cancel", false)),
				layout.Rigid(layout.Spacer{Width: buttonGap}.Layout),
				layout.Rigid(d.button(&c.submit, "Comment", true)),
			)
		}),
	)
}

func (d *demo) posted(gtx layout.Context, c *comment) layout.Dimensions {
	for c.remove.Clicked(gtx) {
		d.dropComment(c)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(d.th, commentText, "you")
					l.Color, l.Font.Weight = d.foreground(), font.SemiBold
					return l.Layout(gtx)
				}),
				layout.Rigid(d.button(&c.remove, "Delete", false)),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Label(d.th, commentText, c.text)
			l.Color = d.foreground()
			return l.Layout(gtx)
		}),
	)
}

// foreground is the views' text color, which the comments share.
func (d *demo) foreground() color.NRGBA {
	return d.theme.Foreground
}

// button is a small rounded button with the hand cursor; primary is the
// filled one a form submits with.
func (d *demo) button(c *widget.Clickable, text string, primary bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		fg, bg, edge := d.foreground(), d.theme.Background, d.theme.Border
		switch {
		case primary:
			fg, bg, edge = d.theme.Background, d.foreground(), d.foreground()
		case c.Hovered():
			bg = d.theme.Diff.AffordanceHover
		}
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return layout.Background{}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					size := gtx.Constraints.Min
					r := gtx.Dp(buttonRadius)
					paint.FillShape(gtx.Ops, edge, clip.UniformRRect(image.Rectangle{Max: size}, r).Op(gtx.Ops))
					paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rect(1, 1, size.X-1, size.Y-1), r-1).Op(gtx.Ops))
					defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
					pointer.CursorPointer.Add(gtx.Ops)
					return layout.Dimensions{Size: size}
				},
				func(gtx layout.Context) layout.Dimensions {
					inset := layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3), Left: buttonPadding, Right: buttonPadding}
					return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						l := material.Label(d.th, buttonTextSize, text)
						l.Color, l.MaxLines = fg, 1
						return l.Layout(gtx)
					})
				},
			)
		})
	}
}

// card lays content out padded on a rounded card with a hairline border.
func (d *demo) card(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			r := gtx.Dp(cardRadius)
			paint.FillShape(gtx.Ops, d.theme.Border, clip.UniformRRect(image.Rectangle{Max: size}, r).Op(gtx.Ops))
			paint.FillShape(gtx.Ops, d.theme.Background, clip.UniformRRect(image.Rect(1, 1, size.X-1, size.Y-1), r-1).Op(gtx.Ops))
			return layout.Dimensions{Size: size}
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(cardPadding).Layout(gtx, content)
		},
	)
}
