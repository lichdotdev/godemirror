package godemirror

import (
	"errors"

	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/glyphs"
)

// ErrNoFontconfig was what New failed with on a machine without fontconfig.
//
// Deprecated: New resolves a family without fontconfig and never fails with
// it; test for ErrFontNotFound instead.
var ErrNoFontconfig = glyphs.ErrNoFontconfig

// ErrFontNotFound is what New fails with, on a machine without fontconfig,
// when no font in the platform's font directories is of the family it was
// asked for. Where fontconfig is installed, fc-match picks its closest font
// instead.
var ErrFontNotFound = glyphs.ErrFontNotFound

// Fonts are the contents of a monospace family's font files (TrueType,
// OpenType or a collection, whose first font is used). Regular is required;
// a style left nil is drawn with Regular.
type Fonts struct {
	Regular, Bold, Italic, BoldItalic []byte
}

// NewFromFonts builds a view drawing fonts at size, without looking for
// installed fonts. A character the fonts do not cover is drawn as a box, since
// there is no installed font to fall back to. It fails when Regular is nil or a style is
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
