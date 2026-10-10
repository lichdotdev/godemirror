package godemirror

import (
	"errors"

	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/glyphs"
)

// ErrNoFontconfig is what New and NewSplit fail with on a machine without
// fontconfig's fc-match, where a family name cannot be resolved to font
// files. NewFromFonts needs no fontconfig.
var ErrNoFontconfig = glyphs.ErrNoFontconfig

// Fonts are the contents of a monospace family's font files (TrueType,
// OpenType or a collection, whose first font is used). Regular is required;
// a style left nil is drawn with Regular.
type Fonts struct {
	Regular, Bold, Italic, BoldItalic []byte
}

// NewFromFonts builds a view drawing fonts at size, with no fontconfig. A
// character the fonts do not cover is drawn as a box, since there is no
// installed font to fall back to. It fails when Regular is nil or a style is
// not a font.
func NewFromFonts(fonts Fonts, size unit.Sp) (*View, error) {
	r, err := rasterizerOf(fonts)
	if err != nil {
		return nil, err
	}
	return newView(r, size), nil
}

func rasterizerOf(fonts Fonts) (*glyphs.Rasterizer, error) {
	if fonts.Regular == nil {
		return nil, errors.New("godemirror: Fonts.Regular is nil")
	}
	styles := [4][]byte{fonts.Regular, fonts.Bold, fonts.Italic, fonts.BoldItalic}
	for i, data := range styles {
		if data == nil {
			styles[i] = fonts.Regular
		}
	}
	return glyphs.NewFromData(styles)
}
