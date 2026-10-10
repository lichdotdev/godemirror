package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// The buttons lich draws inside its diff, .cm-diff-expand and .cm-diff-revert
// (frontend/src/index.css), in its dark theme's colors.
var (
	foreground      = color.NRGBA{R: 0xfa, G: 0xfa, B: 0xfa, A: 0xff}
	mutedForeground = color.NRGBA{R: 0x9f, G: 0x9f, B: 0xa9, A: 0xff}
	accent          = color.NRGBA{R: 0x27, G: 0x27, B: 0x2a, A: 0xff}
	background      = color.NRGBA{R: 0x09, G: 0x09, B: 0x0b, A: 0xff}
	border          = color.NRGBA{R: 0x22, G: 0x22, B: 0x24, A: 0xff}
)

const (
	buttonText    = unit.Sp(11)
	buttonPadding = unit.Dp(6)
	buttonRadius  = unit.Dp(4)
	buttonGap     = unit.Dp(4)
	revertIcon    = unit.Dp(12)
)

// buttons owns the clickables behind every button, keyed by what they act
// on, so a button keeps its state across the rebuilds its own clicks cause.
type buttons struct {
	th     *material.Theme
	undo   *widget.Icon
	expand map[int]*widget.Clickable
	revert map[int]*widget.Clickable
}

func newButtons() (*buttons, error) {
	undo, err := widget.NewIcon(icons.ContentUndo)
	if err != nil {
		return nil, err
	}
	return &buttons{
		th:     material.NewTheme(),
		undo:   undo,
		expand: map[int]*widget.Clickable{},
		revert: map[int]*widget.Clickable{},
	}, nil
}

func clickable(m map[int]*widget.Clickable, key int) *widget.Clickable {
	if c, ok := m[key]; ok {
		return c
	}
	c := new(widget.Clickable)
	m[key] = c
	return c
}

// expandButton offers the lines a gap left out.
func (b *buttons) expandButton(g *gap, onClick func()) layout.Widget {
	c := clickable(b.expand, g.key)
	count := g.to - g.from + 1
	text := fmt.Sprintf("Expand %d unchanged lines", count)
	if count == 1 {
		text = "Expand 1 unchanged line"
	}
	return func(gtx layout.Context) layout.Dimensions {
		for c.Clicked(gtx) {
			onClick()
		}
		fg, bg := mutedForeground, color.NRGBA{}
		if c.Hovered() {
			fg, bg = foreground, accent
		}
		return b.button(gtx, c, bg, color.NRGBA{}, func(gtx layout.Context) layout.Dimensions {
			return b.label(gtx, text, fg)
		})
	}
}

// revertButton undoes one block of changes.
func (b *buttons) revertButton(block int, onClick func()) layout.Widget {
	c := clickable(b.revert, block)
	return func(gtx layout.Context) layout.Dimensions {
		for c.Clicked(gtx) {
			onClick()
		}
		bg := background
		if c.Hovered() {
			bg = accent
		}
		return b.button(gtx, c, bg, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					size := gtx.Dp(revertIcon)
					gtx.Constraints = layout.Exact(image.Pt(size, size))
					return b.undo.Layout(gtx, foreground)
				}),
				layout.Rigid(layout.Spacer{Width: buttonGap}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return b.label(gtx, "Revert", foreground)
				}),
			)
		})
	}
}

func (b *buttons) label(gtx layout.Context, text string, fg color.NRGBA) layout.Dimensions {
	l := material.Label(b.th, buttonText, text)
	l.Color, l.MaxLines = fg, 1
	return l.Layout(gtx)
}

// button lays content out padded on a rounded background with an optional
// border, as the target of c, with the hand cursor.
func (b *buttons) button(gtx layout.Context, c *widget.Clickable, bg, edge color.NRGBA, content layout.Widget) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Min
				shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(buttonRadius))
				if edge.A != 0 {
					paint.FillShape(gtx.Ops, edge, shape.Op(gtx.Ops))
					inner := clip.UniformRRect(image.Rect(1, 1, size.X-1, size.Y-1), gtx.Dp(buttonRadius)-1)
					paint.FillShape(gtx.Ops, bg, inner.Op(gtx.Ops))
				} else if bg.A != 0 {
					paint.FillShape(gtx.Ops, bg, shape.Op(gtx.Ops))
				}
				defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
				pointer.CursorPointer.Add(gtx.Ops)
				return layout.Dimensions{Size: size}
			},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: buttonPadding, Right: buttonPadding}.Layout(gtx, content)
			},
		)
	})
}
