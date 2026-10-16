package godemirror

import (
	"image/color"

	"github.com/lichdotdev/godemirror/tags"
)

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
	// Foreground is the code's color where Highlight gives it none, as the
	// color of CodeMirror's editor theme is.
	Foreground color.NRGBA
	// Highlight styles the code by its highlighting tags; nil draws it all in
	// Foreground.
	Highlight *HighlightStyle
	// SearchMatch tints every match of the search (SetSearch), and
	// SearchCurrent the one that is selected, the current match. Like
	// Selection they may be translucent: they are mixed over the line's
	// background in sRGB. Zero highlights nothing.
	SearchMatch, SearchCurrent color.NRGBA
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
// GutterBackground, and a Strip down that cell's left edge. Changed tints the
// words that changed within the line (DiffOptions.HighlightChanges), over its
// background.
type ChangeColors struct {
	Background, HoverBackground, GutterBackground, Strip, Changed color.NRGBA
}

// The emerald and red of the diff colors, at the percentages a CSS
// color-mix gives them as alpha. ChangeColors.Changed has no CSS to come
// from (the web diff these colors are from has no intra-line highlight): it
// is a step deeper than the hover tint, picked by eye on the demo.
var (
	emerald400 = color.NRGBA{R: 0x00, G: 0xd4, B: 0x92, A: 0xff}
	emerald500 = color.NRGBA{R: 0x00, G: 0xbc, B: 0x7d, A: 0xff}
	red400     = color.NRGBA{R: 0xff, G: 0x64, B: 0x67, A: 0xff}
	red500     = color.NRGBA{R: 0xfb, G: 0x2c, B: 0x36, A: 0xff}
)

// The yellow and the orange of the search colors, after CodeMirror's: a
// yellowish tint for the matches and a stronger one for the selected match.
// The percentages have no CSS to come from; they were picked by eye on the
// demo (-search), so that code reads on both themes.
var (
	amber400  = color.NRGBA{R: 0xfb, G: 0xbf, B: 0x24, A: 0xff}
	orange500 = color.NRGBA{R: 0xf9, G: 0x73, B: 0x16, A: 0xff}
)

func percent(c color.NRGBA, p int) color.NRGBA {
	c.A = uint8(255 * p / 100)
	return c
}

// DarkTheme is the palette a new View starts with: zinc greys under an oklch
// syntax palette, converted to sRGB. A theme of your own starts as a copy of
// DarkTheme or LightTheme with the fields you want changed.
var DarkTheme = Theme{
	Background:       rgb(0x09090b),
	Border:           over(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 26}, rgb(0x09090b)),
	Selection:        color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 90},
	SearchMatch:      percent(amber400, 30),
	SearchCurrent:    percent(orange500, 60),
	GutterBackground: rgb(0x09090b),
	GutterForeground: rgb(0x9f9fa9),
	Foreground:       rgb(0xfafafa),
	Highlight: zincHighlight(zincColors{
		keyword: rgb(0xc396f5), str: rgb(0x82cb92), comment: rgb(0x9f9fa9), literal: rgb(0xeca760),
		typeName: rgb(0x71d0d5), property: rgb(0x82b1ed), definition: rgb(0x82b1ed),
		punctuation: rgb(0x9f9fa9), invalid: rgb(0xff6467),
	}),
	Diff: DiffColors{
		Added: ChangeColors{
			Background: percent(emerald400, 14), HoverBackground: percent(emerald500, 20),
			GutterBackground: percent(emerald500, 18), Strip: emerald500,
			Changed: percent(emerald400, 30),
		},
		Deleted: ChangeColors{
			Background: percent(red400, 12), HoverBackground: percent(red500, 17),
			GutterBackground: percent(red500, 15), Strip: red500,
			Changed: percent(red400, 30),
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
	SearchMatch:      percent(amber400, 55),
	SearchCurrent:    percent(orange500, 60),
	GutterBackground: rgb(0xebebed),
	GutterForeground: rgb(0x64636e),
	Foreground:       rgb(0x18181b),
	Highlight: zincHighlight(zincColors{
		keyword: rgb(0x6c329f), str: rgb(0x206b38), comment: rgb(0x64636e), literal: rgb(0x9d5300),
		typeName: rgb(0x006e74), property: rgb(0x17559b), definition: rgb(0x17559b),
		punctuation: rgb(0x64636e), invalid: rgb(0xe7000b),
	}),
	Diff: DiffColors{
		Added: ChangeColors{
			Background: percent(emerald500, 12), HoverBackground: percent(emerald500, 20),
			GutterBackground: percent(emerald500, 18), Strip: emerald500,
			Changed: percent(emerald500, 25),
		},
		Deleted: ChangeColors{
			Background: percent(red500, 10), HoverBackground: percent(red500, 17),
			GutterBackground: percent(red500, 15), Strip: red500,
			Changed: percent(red500, 22),
		},
		Separator:       rgb(0xdadadd),
		Filler:          percent(rgb(0x18181b), 7),
		AffordanceHover: rgb(0xdedee0),
	},
}

// zincColors are the syntax colors of DarkTheme and LightTheme, one per role
// a syntax palette usually tells apart.
type zincColors struct {
	keyword, str, comment, literal, typeName, property, definition, punctuation, invalid color.NRGBA
}

// zincHighlight lays c over the tags in the groups of CodeMirror's
// classHighlighter, with processing instructions under the comment color so
// a preprocessor directive reads as it does in most editors. A builtin value
// takes the type color, as standard(typeName) and standard(className) do by
// falling back, and a parameter's declaration is italic.
func zincHighlight(c zincColors) *HighlightStyle {
	return NewHighlightStyle(
		TagStyle{Tags: []*tags.Tag{tags.Keyword}, Color: c.keyword},
		TagStyle{Tags: []*tags.Tag{tags.String, tags.Regexp, tags.Escape, tags.Special(tags.String)}, Color: c.str},
		TagStyle{Tags: []*tags.Tag{tags.Comment, tags.ProcessingInstruction}, Color: c.comment, Italic: true},
		TagStyle{Tags: []*tags.Tag{tags.Literal, tags.Number, tags.Bool, tags.Null, tags.Atom}, Color: c.literal},
		TagStyle{Tags: []*tags.Tag{tags.TypeName, tags.ClassName, tags.Namespace, tags.Standard(tags.VariableName)}, Color: c.typeName},
		TagStyle{Tags: []*tags.Tag{tags.PropertyName, tags.TagName}, Color: c.property},
		TagStyle{Tags: []*tags.Tag{tags.Function(tags.VariableName)}, Color: c.definition},
		TagStyle{Tags: []*tags.Tag{tags.Definition(tags.Local(tags.VariableName))}, Italic: true},
		TagStyle{Tags: []*tags.Tag{tags.Punctuation, tags.Operator, tags.OperatorKeyword}, Color: c.punctuation},
		TagStyle{Tags: []*tags.Tag{tags.Invalid}, Color: c.invalid},
		TagStyle{Tags: []*tags.Tag{tags.Strong, tags.Heading}, Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Emphasis}, Italic: true},
		TagStyle{Tags: []*tags.Tag{tags.Link}, Underline: true},
		TagStyle{Tags: []*tags.Tag{tags.Strikethrough}, Strikethrough: true},
	)
}

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}
