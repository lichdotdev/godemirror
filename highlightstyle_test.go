package godemirror

import (
	"image"
	"image/color"
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/tags"
)

var (
	fg     = rgb(0x111111)
	red    = rgb(0xff0000)
	blue   = rgb(0x0000ff)
	yellow = rgb(0xffff00)
)

// tagged is tag alone as a set, the way the chroma path tags a token.
func tagged(tag *tags.Tag) *lezer.TagSet { return soleTag(lezer.NewTagSets(), tag) }

// onlyTag is the one tag in set, nil for an untagged cell.
func onlyTag(t *testing.T, set *lezer.TagSet) *tags.Tag {
	t.Helper()
	if set == nil {
		return nil
	}
	if len(set.Tags) != 1 {
		t.Fatalf("cell has tags %v, want one", set.Tags)
	}
	return set.Tags[0]
}

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
		if got := h.style(tagged(c.tag), fg).Color; got != c.want {
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
	if got := h.style(tagged(tags.Link), fg); got.Color != fg || got.Bold || !got.Underline {
		t.Errorf("link = %+v, want only underlined, in the foreground", got)
	}
	if got := h.style(tagged(tags.URL), fg); got.Color != red || !got.Bold {
		t.Errorf("url = %+v, want the first spec", got)
	}
}

// A token with several tags gets every spec they take, merged as CSS
// cascades CodeMirror's classes: in spec order, whatever order the tags
// come in, a later spec's color winning and every bold or italic kept.
func TestSeveralTagsMergeTheirSpecsInSpecOrder(t *testing.T) {
	keyword := TagStyle{Tags: []*tags.Tag{tags.Keyword}, Color: red, Bold: true}
	str := TagStyle{Tags: []*tags.Tag{tags.String}, Color: blue, Italic: true}
	sets := lezer.NewTagSets()
	for _, set := range []*lezer.TagSet{sets.Of(tags.Keyword, tags.String), sets.Of(tags.String, tags.Keyword)} {
		if got := NewHighlightStyle(keyword, str).style(set, fg); got.Color != blue || !got.Bold || !got.Italic {
			t.Errorf("%v with string's spec last = %+v, want blue, bold and italic", set.Tags, got)
		}
		if got := NewHighlightStyle(str, keyword).style(set, fg); got.Color != red || !got.Bold || !got.Italic {
			t.Errorf("%v with keyword's spec last = %+v, want red, bold and italic", set.Tags, got)
		}
	}
}

// Each tag of a set falls back on its own, to the most specific styled tag
// in its Set, before the specs merge: variableName takes name's spec here,
// function(variableName) its own, and the later of the two wins the color.
func TestEachTagOfASetTakesItsMostSpecificSpec(t *testing.T) {
	h := NewHighlightStyle(
		TagStyle{Tags: []*tags.Tag{tags.Function(tags.VariableName)}, Color: blue, Underline: true},
		TagStyle{Tags: []*tags.Tag{tags.Name}, Color: red},
		TagStyle{Tags: []*tags.Tag{tags.Keyword}, Strikethrough: true},
	)
	sets := lezer.NewTagSets()
	cases := []struct {
		set  *lezer.TagSet
		want TagStyle
	}{
		{sets.Of(tags.Function(tags.VariableName), tags.Keyword), TagStyle{Color: blue, Underline: true, Strikethrough: true}},
		{sets.Of(tags.VariableName, tags.Keyword), TagStyle{Color: red, Strikethrough: true}},
		{sets.Of(tags.Function(tags.VariableName), tags.VariableName), TagStyle{Color: red, Underline: true}},
		{sets.Of(tags.Keyword, tags.Comment), TagStyle{Color: fg, Strikethrough: true}},
		{sets.Of(tags.Comment, tags.Meta), TagStyle{Color: fg}},
	}
	for _, c := range cases {
		got := h.style(c.set, fg)
		if got.Color != c.want.Color || got.Bold != c.want.Bold || got.Italic != c.want.Italic ||
			got.Underline != c.want.Underline || got.Strikethrough != c.want.Strikethrough {
			t.Errorf("%v = %+v, want %+v", c.set.Tags, got, c.want)
		}
	}
}

func TestNoHighlightStyleDrawsEverythingInTheForeground(t *testing.T) {
	var h *HighlightStyle
	if got := h.style(tagged(tags.Keyword), fg); got.Color != fg || got.Bold || got.Italic || got.Underline || got.Strikethrough {
		t.Errorf("style = %+v, want plain foreground", got)
	}
}

func styledHarness(t *testing.T, text string, specs ...TagStyle) *harness {
	t.Helper()
	h := newHarness(t, "")
	h.v.Theme.Highlight = NewHighlightStyle(specs...)
	d, err := highlight("a.go", text, nil)
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

func TestSpecsAppendedToAnotherStyleWinOverItsOwn(t *testing.T) {
	h := NewHighlightStyle(append(OneDark.Highlight.Specs(),
		TagStyle{Tags: []*tags.Tag{tags.Keyword}, Color: red, Bold: true})...)
	if got := h.style(tagged(tags.Keyword), fg); got.Color != red || !got.Bold {
		t.Errorf("keyword = %+v, want the appended spec", got)
	}
	want := OneDark.Highlight.style(tagged(tags.String), fg)
	if got := h.style(tagged(tags.String), fg); got.Color != want.Color {
		t.Errorf("string = %v, want OneDark's %v", got.Color, want.Color)
	}
}

func TestChangingSpecsLeavesTheStyleAsItIs(t *testing.T) {
	given := []TagStyle{{Tags: []*tags.Tag{tags.Keyword}, Color: red}}
	h := NewHighlightStyle(given...)
	given[0].Tags[0] = tags.String
	specs := h.Specs()
	specs[0].Tags[0] = tags.Comment
	specs[0].Color = blue
	if got := h.Specs(); got[0].Tags[0] != tags.Keyword || got[0].Color != red {
		t.Errorf("specs = %+v, want the keyword in red they were built from", got)
	}
	if got := h.style(tagged(tags.Keyword), fg); got.Color != red {
		t.Errorf("keyword = %v, want red", got.Color)
	}
}

// The zinc palettes draw a parameter's declaration italic in the text color
// and a builtin type, value or class in the type color; a called builtin
// keeps the function color.
func TestZincThemesStyleParametersAndBuiltins(t *testing.T) {
	for name, c := range map[string]struct {
		theme              Theme
		typeName, function color.NRGBA
	}{
		"DarkTheme":  {DarkTheme, rgb(0x71d0d5), rgb(0x82b1ed)},
		"LightTheme": {LightTheme, rgb(0x006e74), rgb(0x17559b)},
	} {
		h, text := c.theme.Highlight, c.theme.Foreground
		if got := h.style(tagged(tags.Definition(tags.Local(tags.VariableName))), text); got.Color != text || !got.Italic || got.Bold {
			t.Errorf("%s: parameter %+v, want italic in the text color", name, got)
		}
		for _, tag := range []*tags.Tag{tags.Standard(tags.TypeName), tags.Standard(tags.VariableName), tags.Standard(tags.ClassName)} {
			if got := h.style(tagged(tag), text); got.Color != c.typeName || got.Italic || got.Bold {
				t.Errorf("%s: %v %+v, want the type color", name, tag, got)
			}
		}
		if got := h.style(tagged(tags.Function(tags.Standard(tags.VariableName))), text); got.Color != c.function || got.Italic || got.Bold {
			t.Errorf("%s: called builtin %+v, want the function color", name, got)
		}
	}
}
