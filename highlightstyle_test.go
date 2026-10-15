package godemirror

import (
	"image"
	"image/color"
	"testing"

	"github.com/lichdotdev/godemirror/tags"
)

var (
	fg     = rgb(0x111111)
	red    = rgb(0xff0000)
	blue   = rgb(0x0000ff)
	yellow = rgb(0xffff00)
)

func TestATagFallsBackToTheMostSpecificStyledTag(t *testing.T) {
	h := NewHighlightStyle(
		TagStyle{Tags: []*tags.Tag{tags.Name}, Color: red},
		TagStyle{Tags: []*tags.Tag{tags.Function(tags.VariableName)}, Color: blue},
	)
	cases := []struct {
		tag  *tags.Tag
		want color.NRGBA
	}{
		{tags.Definition(tags.Function(tags.VariableName)), blue},
		{tags.Function(tags.VariableName), blue},
		{tags.VariableName, red},
		{tags.AttributeName, red},
		{tags.Keyword, fg},
		{nil, fg},
	}
	for _, c := range cases {
		if got := h.style(c.tag, fg).Color; got != c.want {
			t.Errorf("%v: color %v, want %v", c.tag, got, c.want)
		}
	}
}

// CodeMirror keys a highlighter by tag, so a later spec for the same tag
// replaces the earlier one whole: oneDark's link loses the cyan its first
// spec gave it and keeps only the stone and the underline of its second.
func TestALaterSpecForTheSameTagReplacesTheEarlierOne(t *testing.T) {
	h := NewHighlightStyle(
		TagStyle{Tags: []*tags.Tag{tags.URL, tags.Link}, Color: red, Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Link}, Underline: true},
	)
	if got := h.style(tags.Link, fg); got.Color != fg || got.Bold || !got.Underline {
		t.Errorf("link = %+v, want only underlined, in the foreground", got)
	}
	if got := h.style(tags.URL, fg); got.Color != red || !got.Bold {
		t.Errorf("url = %+v, want the first spec", got)
	}
}

func TestNoHighlightStyleDrawsEverythingInTheForeground(t *testing.T) {
	var h *HighlightStyle
	if got := h.style(tags.Keyword, fg); got.Color != fg || got.Bold || got.Italic || got.Underline || got.Strikethrough {
		t.Errorf("style = %+v, want plain foreground", got)
	}
}

func styledHarness(t *testing.T, text string, specs ...TagStyle) *harness {
	t.Helper()
	h := newHarness(t, "")
	h.v.Theme.Highlight = NewHighlightStyle(specs...)
	d, err := highlight("a.go", text)
	if err != nil {
		t.Fatal(err)
	}
	h.v.doc = d
	return h
}

// litIn reports whether any pixel of img inside band is drawn opaque in c.
func litIn(img *image.RGBA, band image.Rectangle, c color.NRGBA) bool {
	for y := band.Min.Y; y < band.Max.Y; y++ {
		for x := band.Min.X; x < band.Max.X; x++ {
			if p := img.RGBAAt(x, y); p.R == c.R && p.G == c.G && p.B == c.B && p.A == 0xff {
				return true
			}
		}
	}
	return false
}

func TestAnUnderlinedTokenIsRuledUnderEveryCellSpacesIncluded(t *testing.T) {
	h := styledHarness(t, "// a b\nx",
		TagStyle{Tags: []*tags.Tag{tags.Comment}, Color: yellow, Underline: true})
	img := h.v.rasterize(h.v.doc.lines[0], h.v.doc.tags[0])
	cw, rule := h.v.r.CellW(), h.v.r.Underline()
	for cell := range len(h.v.doc.lines[0]) {
		band := image.Rect(cell*cw, rule.Y, (cell+1)*cw, rule.Y+rule.Thickness)
		if !litIn(img, band, yellow) {
			t.Fatalf("cell %d of the comment has no underline", cell)
		}
	}
	plain := h.v.rasterize(h.v.doc.lines[1], h.v.doc.tags[1])
	if litIn(plain, image.Rect(0, rule.Y, cw, rule.Y+rule.Thickness), yellow) {
		t.Fatal("an unstyled token is underlined")
	}
}

func TestAStruckTokenIsRuledAcrossItsMiddle(t *testing.T) {
	h := styledHarness(t, "x",
		TagStyle{Tags: []*tags.Tag{tags.VariableName}, Color: yellow, Strikethrough: true})
	img := h.v.rasterize(h.v.doc.lines[0], h.v.doc.tags[0])
	rule := h.v.r.Strikethrough()
	if rule.Y <= 0 || rule.Y >= h.v.r.Underline().Y {
		t.Fatalf("strikethrough at %d, want between the cell's top and the underline at %d", rule.Y, h.v.r.Underline().Y)
	}
	full := image.Rect(0, rule.Y, h.v.r.CellW(), rule.Y+rule.Thickness)
	for x := full.Min.X; x < full.Max.X; x++ {
		if !litIn(img, image.Rect(x, full.Min.Y, x+1, full.Max.Y), yellow) {
			t.Fatalf("column %d of the line-through is not drawn", x)
		}
	}
}

func TestABoldTokenIsDrawnInTheBoldFace(t *testing.T) {
	regular := styledHarness(t, "func", TagStyle{Tags: []*tags.Tag{tags.Keyword}, Color: yellow})
	bold := styledHarness(t, "func", TagStyle{Tags: []*tags.Tag{tags.Keyword}, Color: yellow, Bold: true})
	count := func(h *harness) int {
		img := h.v.rasterize(h.v.doc.lines[0], h.v.doc.tags[0])
		n := 0
		for i := 3; i < len(img.Pix); i += 4 {
			n += int(img.Pix[i])
		}
		return n
	}
	if r, b := count(regular), count(bold); b <= r {
		t.Fatalf("bold covers %d, regular %d: want bold heavier", b, r)
	}
}
