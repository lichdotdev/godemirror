package godemirror

import (
	"image/color"
	"slices"

	"github.com/lichdotdev/godemirror/internal/lezer"
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

// HighlightStyle maps highlighting tags to styles, as CodeMirror's does. Each
// of a token's tags takes the spec of the most specific tag in its Set that
// has one. A token can have several tags (a grammar gives some nodes more
// than one, and a node's tags can apply to everything inside it): it is drawn
// with every spec its tags take, merged in spec order, a later spec's color,
// bold, italic, underline or line-through winning over an earlier one's, as
// CSS cascades the classes CodeMirror gives such a token. A token no style
// reaches is drawn plain in Theme.Foreground. A HighlightStyle is immutable,
// so a Theme holding it compares equal exactly while it draws the same.
type HighlightStyle struct {
	specs []TagStyle
	// specOf is the index in specs of each styled tag's spec.
	specOf map[*tags.Tag]int
}

// NewHighlightStyle builds a HighlightStyle from specs, in CodeMirror's
// order: when two specs name the same tag, the later one is the tag's style
// and the earlier one no longer applies to it.
func NewHighlightStyle(specs ...TagStyle) *HighlightStyle {
	h := &HighlightStyle{specs: cloneSpecs(specs), specOf: map[*tags.Tag]int{}}
	for i, s := range h.specs {
		for _, t := range s.Tags {
			h.specOf[t] = i
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

// style is how a token tagged with set is drawn, with fg for a style that
// sets no color. A nil h or set is plain.
func (h *HighlightStyle) style(set *lezer.TagSet, fg color.NRGBA) TagStyle {
	merged := TagStyle{}
	if h != nil && set != nil {
		var found [maxSetTags]int
		for _, i := range h.specsOf(set, found[:0]) {
			merged = merged.over(h.specs[i])
		}
	}
	if merged.Color == (color.NRGBA{}) {
		merged.Color = fg
	}
	return merged
}

// maxSetTags is room for the tags of a typical set, so resolving one per
// cell does not allocate; a bigger set still works.
const maxSetTags = 8

// specsOf appends to found the indices of the specs set's tags take, in
// spec order.
func (h *HighlightStyle) specsOf(set *lezer.TagSet, found []int) []int {
	for _, t := range set.Tags {
		for sub := range t.Set() {
			if i, ok := h.specOf[sub]; ok {
				found = append(found, i)
				break
			}
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

// over is s with what later sets on top of it, as a later CSS rule
// overrides the properties it sets.
func (s TagStyle) over(later TagStyle) TagStyle {
	if later.Color != (color.NRGBA{}) {
		s.Color = later.Color
	}
	s.Bold = s.Bold || later.Bold
	s.Italic = s.Italic || later.Italic
	s.Underline = s.Underline || later.Underline
	s.Strikethrough = s.Strikethrough || later.Strikethrough
	return s
}
