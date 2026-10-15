package godemirror

import (
	"image/color"
	"slices"

	"github.com/lichdotdev/godemirror/tags"
)

// TagStyle is how the code under some tags is drawn, as one spec of
// CodeMirror's HighlightStyle.define: a color, the CSS fontWeight bold and
// fontStyle italic, and the textDecoration underline and line-through. A zero
// Color leaves the text in Theme.Foreground, the way a CSS rule with no color
// leaves the editor's.
type TagStyle struct {
	Tags                                   []*tags.Tag
	Color                                  color.NRGBA
	Bold, Italic, Underline, Strikethrough bool
}

// HighlightStyle maps highlighting tags to styles, as CodeMirror's does. A
// token takes the style of the most specific tag in its tag's Set that has
// one, and a token no style reaches is drawn plain in Theme.Foreground. It is
// immutable, so a Theme holding it compares equal exactly while it draws the
// same.
type HighlightStyle struct {
	specs []TagStyle
	byTag map[*tags.Tag]TagStyle
}

// NewHighlightStyle builds a HighlightStyle from specs, in CodeMirror's
// order: when two specs name the same tag, the later one is the tag's style
// and the earlier one no longer applies to it.
func NewHighlightStyle(specs ...TagStyle) *HighlightStyle {
	h := &HighlightStyle{specs: cloneSpecs(specs), byTag: map[*tags.Tag]TagStyle{}}
	for _, s := range h.specs {
		for _, t := range s.Tags {
			h.byTag[t] = s
		}
	}
	return h
}

// Specs are the specs h was built from, in their order: a copy, so changing
// them leaves h as it is. A theme of your own can start from another's style
// with specs of its own appended, which win over the ones they name the same
// tags as:
//
//	t := godemirror.OneDark
//	t.Highlight = godemirror.NewHighlightStyle(append(t.Highlight.Specs(),
//		godemirror.TagStyle{Tags: []*tags.Tag{tags.Keyword}, Color: pink, Bold: true})...)
func (h *HighlightStyle) Specs() []TagStyle {
	return cloneSpecs(h.specs)
}

func cloneSpecs(specs []TagStyle) []TagStyle {
	out := slices.Clone(specs)
	for i := range out {
		out[i].Tags = slices.Clone(out[i].Tags)
	}
	return out
}

// style is how a token tagged t is drawn, with fg for a style that sets no
// color. A nil h or t is plain.
func (h *HighlightStyle) style(t *tags.Tag, fg color.NRGBA) TagStyle {
	plain := TagStyle{Color: fg}
	if h == nil || t == nil {
		return plain
	}
	for sub := range t.Set() {
		s, ok := h.byTag[sub]
		if !ok {
			continue
		}
		if s.Color == (color.NRGBA{}) {
			s.Color = fg
		}
		return s
	}
	return plain
}
