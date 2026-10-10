package godemirror

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// gutterStripWidth is the colored bar a decorated line's gutter cell carries,
// 3px, like the inset shadow a web diff's gutter draws for a changed line.
const gutterStripWidth = unit.Dp(3)

// LineDecoration styles one document line across every visual row it wraps
// into. A zero color draws nothing; a translucent one is mixed over the
// theme's background in sRGB, the way a browser mixes CSS colors.
type LineDecoration struct {
	// Background tints the code area behind the line.
	Background color.NRGBA
	// GutterBackground tints the line's gutter cell.
	GutterBackground color.NRGBA
	// GutterStrip is a bar down the left edge of the gutter cell.
	GutterStrip color.NRGBA
	// Rule is a hairline through the middle of the line, across the code area.
	Rule color.NRGBA
	// Hatch strikes thin diagonal lines across the code area: the row a
	// side-by-side diff stands in for a line only the other side has.
	Hatch color.NRGBA
	// HoverGroup groups lines that light up together: while the pointer is
	// over any line of a group, every line of it takes HoverBackground in
	// place of Background. 0 is no group.
	HoverGroup int
	// HoverBackground replaces Background while the line's group is under
	// the pointer; zero keeps Background.
	HoverBackground color.NRGBA
	// Spans tint stretches of the line's characters over its background:
	// the words that changed in a diff, say.
	Spans []Span

	// role, set only by a DiffView, takes the colors from the view's
	// Theme.Diff when the line is painted, so a theme change needs no new
	// content.
	role diffRole
}

type diffRole uint8

const (
	roleNone diffRole = iota
	roleAdded
	roleDeleted
	roleSeparator
	roleFiller
)

// themed is d with the colors its role takes from c.
func (d LineDecoration) themed(c DiffColors) LineDecoration {
	change := func(cc ChangeColors) {
		d.Background, d.HoverBackground = cc.Background, cc.HoverBackground
		d.GutterBackground, d.GutterStrip = cc.GutterBackground, cc.Strip
		if len(d.Spans) == 0 {
			return
		}
		spans := make([]Span, len(d.Spans))
		for i, s := range d.Spans {
			spans[i] = Span{s.From, s.To, cc.Changed}
		}
		d.Spans = spans
	}
	switch d.role {
	case roleAdded:
		change(c.Added)
	case roleDeleted:
		change(c.Deleted)
	case roleSeparator:
		d.Rule = c.Separator
	case roleFiller:
		d.Hatch = c.Filler
	}
	return d
}

// hatchPeriod is the spacing of the hatch's lines along a row: a CSS 6px
// repeating gradient at 45°, so 6·√2 across.
const hatchPeriod = 8

func (v *View) decoration(line int) LineDecoration {
	if line >= 0 && line < len(v.content.Decorations) {
		return v.content.Decorations[line].themed(v.Theme.Diff)
	}
	return LineDecoration{}
}

func (v *View) paintLineBackgrounds(gtx layout.Context, rows []visibleRow, code image.Rectangle, ch int) {
	for _, r := range rows {
		d := v.decoration(r.line)
		row := image.Rect(code.Min.X, r.y, code.Max.X, r.y+ch)
		fill(gtx, v.Theme.Background, v.background(d), row)
		mid := r.y + ch/2
		fill(gtx, v.Theme.Background, d.Rule, image.Rect(code.Min.X, mid, code.Max.X, mid+1))
		if d.Hatch.A != 0 {
			img := v.hatch(code.Dx(), ch, (r.y+v.scroll.Y)%hatchPeriod, over(d.Hatch, v.Theme.Background))
			v.paintImage(gtx, img, row.Min, code)
		}
	}
}

// background is what d tints its line with now: HoverBackground while its
// group is under the pointer.
func (v *View) background(d LineDecoration) color.NRGBA {
	if d.HoverGroup != 0 && d.HoverGroup == v.hot && d.HoverBackground.A != 0 {
		return d.HoverBackground
	}
	return d.Background
}

type hatchKey struct {
	width, height, phase int
	color                color.NRGBA
}

// hatch returns a row of the hatch pattern. phase is the row's offset into the
// pattern, so the lines run on unbroken from one row into the next.
func (v *View) hatch(width, height, phase int, c color.NRGBA) *rowImage {
	k := hatchKey{width, height, phase, c}
	if img, ok := v.hatches[k]; ok {
		img.used = v.frameNo
		return img
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			if (x+y+phase)%hatchPeriod == 0 {
				dst.SetNRGBA(x, y, c)
			}
		}
	}
	o := paint.NewImageOp(dst)
	o.Filter = paint.FilterNearest
	img := &rowImage{op: o, size: dst.Bounds().Size(), used: v.frameNo}
	v.hatches[k] = img
	return img
}

func (v *View) paintGutterDecorations(gtx layout.Context, rows []visibleRow, gutterW, ch int) {
	for _, r := range rows {
		d := v.decoration(r.line)
		base := v.Theme.GutterBackground
		fill(gtx, base, d.GutterBackground, image.Rect(0, r.y, gutterW, r.y+ch))
		fill(gtx, base, d.GutterStrip, image.Rect(0, r.y, gtx.Dp(gutterStripWidth), r.y+ch))
	}
}

// fill paints c over base. Gio blends translucent colors in linear light,
// where a 14% tint lands far stronger than the same tint in CSS, so c is
// mixed over base in sRGB here and painted opaque.
func fill(gtx layout.Context, base, c color.NRGBA, r image.Rectangle) {
	if c.A == 0 {
		return
	}
	paint.FillShape(gtx.Ops, over(c, base), clip.Rect(r).Op())
}

// over mixes c over an opaque base in sRGB.
func over(c, base color.NRGBA) color.NRGBA {
	mix := func(top, bottom uint8) uint8 {
		return uint8((int(top)*int(c.A) + int(bottom)*(255-int(c.A)) + 127) / 255)
	}
	return color.NRGBA{R: mix(c.R, base.R), G: mix(c.G, base.G), B: mix(c.B, base.B), A: 0xff}
}
