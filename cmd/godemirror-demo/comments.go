package main

import (
	"image"
	"image/color"
	"slices"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/godemirror"
)

// thread is a review comment on a line, first as the box it is typed into and
// then, posted, as the comment itself.
type thread struct {
	// anchor is the diffLine.id of the line it is about.
	anchor int
	editor widget.Editor
	// text is what was posted; "" while it is still being written.
	text   string
	focus  bool
	cancel widget.Clickable
	submit widget.Clickable
	remove widget.Clickable
}

// The box around a thread: lich's .cm-thread-slot padding and its card.
var (
	card = color.NRGBA{R: 0x18, G: 0x18, B: 0x1b, A: 0xff}
)

const (
	slotPaddingX  = unit.Dp(8)
	slotPaddingY  = unit.Dp(6)
	cardPadding   = unit.Dp(10)
	cardRadius    = unit.Dp(6)
	bodyText      = unit.Sp(13)
	composerLines = 3
)

// openComposer opens a comment box under the last line of the selection, on
// whichever side of the diff holds it.
func (d *demo) openComposer() {
	ids, line, ok := d.selectedLine()
	if !ok || ids[line] == 0 {
		return
	}
	t := &thread{anchor: ids[line], focus: true}
	d.threads = append(d.threads, t)
	d.rebuild()
}

func (d *demo) selectedLine() (ids []int, line int, ok bool) {
	if !d.showSplit {
		_, line, ok = d.unified.SelectedLines()
		return d.sides.unified.ids, line, ok
	}
	if _, line, ok = d.split.Right.SelectedLines(); ok {
		return d.sides.right.ids, line, ok
	}
	_, line, ok = d.split.Left.SelectedLines()
	return d.sides.left.ids, line, ok
}

func (d *demo) dropThread(t *thread) {
	d.threads = slices.DeleteFunc(d.threads, func(o *thread) bool { return o == t })
	d.rebuild()
}

// threadBlocks places the threads whose line is on s. skip leaves out those
// another side already shows.
func (d *demo) threadBlocks(s side, skip func(anchor int) bool) []godemirror.BlockWidget {
	var blocks []godemirror.BlockWidget
	for _, t := range d.threads {
		line := slices.Index(s.ids, t.anchor)
		if line < 0 || skip(t.anchor) {
			continue
		}
		blocks = append(blocks, godemirror.BlockWidget{After: line, Widget: d.threadWidget(t)})
	}
	return blocks
}

func (d *demo) threadWidget(t *thread) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		inset := layout.Inset{Top: slotPaddingY, Bottom: slotPaddingY, Left: slotPaddingX, Right: slotPaddingX}
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return cardLayout(gtx, func(gtx layout.Context) layout.Dimensions {
				if t.text == "" {
					return d.composer(gtx, t)
				}
				return d.posted(gtx, t)
			})
		})
	}
}

func (d *demo) composer(gtx layout.Context, t *thread) layout.Dimensions {
	if t.focus {
		gtx.Execute(key.FocusCmd{Tag: &t.editor})
		t.focus = false
	}
	for t.cancel.Clicked(gtx) {
		d.dropThread(t)
	}
	for t.submit.Clicked(gtx) {
		if t.editor.Text() != "" {
			t.text = t.editor.Text()
		}
	}
	th := d.buttons.th
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			e := material.Editor(th, &t.editor, "Leave a comment")
			e.TextSize, e.Color, e.HintColor = bodyText, foreground, mutedForeground
			gtx.Constraints.Min.Y = gtx.Sp(bodyText) * composerLines * 3 / 2
			return e.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: cardPadding}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
				layout.Rigid(d.textButton(&t.cancel, "Cancel", false)),
				layout.Rigid(layout.Spacer{Width: buttonGap}.Layout),
				layout.Rigid(d.textButton(&t.submit, "Comment", true)),
			)
		}),
	)
}

func (d *demo) posted(gtx layout.Context, t *thread) layout.Dimensions {
	for t.remove.Clicked(gtx) {
		d.dropThread(t)
	}
	th := d.buttons.th
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					l := material.Label(th, bodyText, "you")
					l.Color, l.Font.Weight = foreground, font.SemiBold
					return l.Layout(gtx)
				}),
				layout.Rigid(d.textButton(&t.remove, "Delete", false)),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Label(th, bodyText, t.text)
			l.Color = foreground
			return l.Layout(gtx)
		}),
	)
}

// textButton is a small button; primary is the filled one a form submits with.
func (d *demo) textButton(c *widget.Clickable, text string, primary bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		fg, bg, edge := foreground, color.NRGBA{}, border
		if primary {
			fg, bg, edge = background, foreground, color.NRGBA{}
		} else if c.Hovered() {
			bg = accent
		}
		return d.buttons.button(gtx, c, bg, edge, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(3)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return d.buttons.label(gtx, text, fg)
			})
		})
	}
}

// cardLayout draws content padded on a bordered, rounded card.
func cardLayout(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			r := gtx.Dp(cardRadius)
			paint.FillShape(gtx.Ops, border, clip.UniformRRect(image.Rectangle{Max: size}, r).Op(gtx.Ops))
			paint.FillShape(gtx.Ops, card, clip.UniformRRect(image.Rect(1, 1, size.X-1, size.Y-1), r-1).Op(gtx.Ops))
			return layout.Dimensions{Size: size}
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(cardPadding).Layout(gtx, content)
		},
	)
}
