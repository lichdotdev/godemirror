package godemirror

import "image/color"

// Theme is the view's palette. Selection may be translucent, as may the
// colors of a LineDecoration: they are mixed over Background in sRGB, the way
// a browser mixes CSS colors. The other colors are painted as they are and
// should be opaque, since Gio blends alpha in linear light.
type Theme struct {
	// Background fills the code area.
	Background color.NRGBA
	// Selection fills the selected cells.
	Selection color.NRGBA
	// Border divides the two sides of a Split.
	Border color.NRGBA
	// GutterBackground fills the gutter, and GutterForeground draws the line
	// numbers or Content.Labels in it.
	GutterBackground color.NRGBA
	GutterForeground color.NRGBA
	// Syntax styles each TokenClass, indexed by it; Plain is the text's own
	// color.
	Syntax [NumTokenClasses]TokenStyle
	// Diff colors a DiffView's changes and the rows around them.
	Diff DiffColors
}

// DiffColors are the colors of a diff. Like a LineDecoration's, each may be
// translucent and is mixed over the theme's background.
type DiffColors struct {
	Added, Deleted ChangeColors
	// Separator rules the row standing for lines a diff leaves out, when it
	// offers nothing to expand them with.
	Separator color.NRGBA
	// Filler hatches the row that, side by side, stands for a line only the
	// other side has.
	Filler color.NRGBA
	// AffordanceHover is the background of the default expand and revert
	// affordances under the pointer. Their text takes GutterForeground.
	AffordanceHover color.NRGBA
}

// ChangeColors are how an added or a deleted line is drawn: its Background,
// and HoverBackground while the pointer is over its chunk; its gutter cell's
// GutterBackground, and a Strip down that cell's left edge.
type ChangeColors struct {
	Background, HoverBackground, GutterBackground, Strip color.NRGBA
}

// The emerald and red of the diff colors, at the percentages a CSS
// color-mix gives them as alpha.
var (
	emerald400 = color.NRGBA{R: 0x00, G: 0xd4, B: 0x92, A: 0xff}
	emerald500 = color.NRGBA{R: 0x00, G: 0xbc, B: 0x7d, A: 0xff}
	red400     = color.NRGBA{R: 0xff, G: 0x64, B: 0x67, A: 0xff}
	red500     = color.NRGBA{R: 0xfb, G: 0x2c, B: 0x36, A: 0xff}
)

func percent(c color.NRGBA, p int) color.NRGBA {
	c.A = uint8(255 * p / 100)
	return c
}

// TokenStyle is how one TokenClass is drawn: its color, and whether in the
// family's italic.
type TokenStyle struct {
	Color  color.NRGBA
	Italic bool
}

// DarkTheme is the palette a new View starts with: zinc greys under an oklch
// syntax palette, converted to sRGB. A theme of your own starts as a copy of
// DarkTheme or LightTheme with the fields you want changed.
var DarkTheme = Theme{
	Background:       rgb(0x09090b),
	Border:           over(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 26}, rgb(0x09090b)),
	Selection:        color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 90},
	GutterBackground: rgb(0x09090b),
	GutterForeground: rgb(0x9f9fa9),
	Syntax: [NumTokenClasses]TokenStyle{
		Plain:       {Color: rgb(0xfafafa)},
		Keyword:     {Color: rgb(0xc396f5)},
		String:      {Color: rgb(0x82cb92)},
		Comment:     {Color: rgb(0x9f9fa9), Italic: true},
		Literal:     {Color: rgb(0xeca760)},
		TypeName:    {Color: rgb(0x71d0d5)},
		Property:    {Color: rgb(0x82b1ed)},
		Definition:  {Color: rgb(0x82b1ed)},
		Punctuation: {Color: rgb(0x9f9fa9)},
		Invalid:     {Color: rgb(0xff6467)},
	},
	Diff: DiffColors{
		Added: ChangeColors{
			Background: percent(emerald400, 14), HoverBackground: percent(emerald500, 20),
			GutterBackground: percent(emerald500, 18), Strip: emerald500,
		},
		Deleted: ChangeColors{
			Background: percent(red400, 12), HoverBackground: percent(red500, 17),
			GutterBackground: percent(red500, 15), Strip: red500,
		},
		Separator:       percent(rgb(0xffffff), 10),
		Filler:          percent(rgb(0xffffff), 7),
		AffordanceHover: rgb(0x27272a),
	},
}

// LightTheme is DarkTheme's light counterpart, from the same oklch palette.
// Its selection is DarkTheme's blue.
var LightTheme = Theme{
	Background:       rgb(0xebebed),
	Border:           rgb(0xdadadd),
	Selection:        color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 90},
	GutterBackground: rgb(0xebebed),
	GutterForeground: rgb(0x64636e),
	Syntax: [NumTokenClasses]TokenStyle{
		Plain:       {Color: rgb(0x18181b)},
		Keyword:     {Color: rgb(0x6c329f)},
		String:      {Color: rgb(0x206b38)},
		Comment:     {Color: rgb(0x64636e), Italic: true},
		Literal:     {Color: rgb(0x9d5300)},
		TypeName:    {Color: rgb(0x006e74)},
		Property:    {Color: rgb(0x17559b)},
		Definition:  {Color: rgb(0x17559b)},
		Punctuation: {Color: rgb(0x64636e)},
		Invalid:     {Color: rgb(0xe7000b)},
	},
	Diff: DiffColors{
		Added: ChangeColors{
			Background: percent(emerald500, 12), HoverBackground: percent(emerald500, 20),
			GutterBackground: percent(emerald500, 18), Strip: emerald500,
		},
		Deleted: ChangeColors{
			Background: percent(red500, 10), HoverBackground: percent(red500, 17),
			GutterBackground: percent(red500, 15), Strip: red500,
		},
		Separator:       rgb(0xdadadd),
		Filler:          percent(rgb(0x18181b), 7),
		AffordanceHover: rgb(0xdedee0),
	},
}

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}
