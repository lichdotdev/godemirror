package godemirror

import (
	"image/color"

	"github.com/lichdotdev/godemirror/tags"
)

// The themes below are CodeMirror themes ported spec for spec: the editor
// theme's background, text, selection, gutter and search colors, and the
// HighlightStyle with its tags, colors and font styles in their order. Where
// a theme leaves a color unset, it takes the one CodeMirror's own base theme
// would show (@codemirror/view for the gutter, @codemirror/search for
// matches). What CodeMirror draws and godemirror does not (the cursor, the
// active line, a match's outline, a token's background or border) is left
// out. A theme has no divider for a Split nor colors for a diff of its own:
// those come from cmTheme.

// OneDark is @codemirror/theme-one-dark 6.1.3.
var OneDark = cmTheme(true, Theme{
	Background:       rgb(0x282c34),
	Foreground:       rgb(0xabb2bf),
	Selection:        rgb(0x3e4451),
	GutterBackground: rgb(0x282c34),
	GutterForeground: rgb(0x7d8799),
	SearchMatch:      rgba(0x72a1ff59),
	SearchCurrent:    rgba(0x6199ff2f),
	Highlight: func() *HighlightStyle {
		chalky, coral, cyan, ivory := rgb(0xe5c07b), rgb(0xe06c75), rgb(0x56b6c2), rgb(0xabb2bf)
		stone, malibu, sage, whiskey := rgb(0x7d8799), rgb(0x61afef), rgb(0x98c379), rgb(0xd19a66)
		violet, invalid := rgb(0xc678dd), rgb(0xffffff)
		return NewHighlightStyle(
			TagStyle{Tags: []*tags.Tag{tags.Keyword}, Color: violet},
			TagStyle{Tags: []*tags.Tag{tags.Name, tags.Deleted, tags.Character, tags.PropertyName, tags.MacroName}, Color: coral},
			TagStyle{Tags: []*tags.Tag{tags.Function(tags.VariableName), tags.LabelName}, Color: malibu},
			TagStyle{Tags: []*tags.Tag{tags.Color, tags.Constant(tags.Name), tags.Standard(tags.Name)}, Color: whiskey},
			TagStyle{Tags: []*tags.Tag{tags.Definition(tags.Name), tags.Separator}, Color: ivory},
			TagStyle{Tags: []*tags.Tag{tags.TypeName, tags.ClassName, tags.Number, tags.Changed, tags.Annotation, tags.Modifier, tags.Self, tags.Namespace}, Color: chalky},
			TagStyle{Tags: []*tags.Tag{tags.Operator, tags.OperatorKeyword, tags.URL, tags.Escape, tags.Regexp, tags.Link, tags.Special(tags.String)}, Color: cyan},
			TagStyle{Tags: []*tags.Tag{tags.Meta, tags.Comment}, Color: stone},
			TagStyle{Tags: []*tags.Tag{tags.Strong}, Bold: true},
			TagStyle{Tags: []*tags.Tag{tags.Emphasis}, Italic: true},
			TagStyle{Tags: []*tags.Tag{tags.Strikethrough}, Strikethrough: true},
			TagStyle{Tags: []*tags.Tag{tags.Link}, Color: stone, Underline: true},
			TagStyle{Tags: []*tags.Tag{tags.Heading}, Color: coral, Bold: true},
			TagStyle{Tags: []*tags.Tag{tags.Atom, tags.Bool, tags.Special(tags.VariableName)}, Color: whiskey},
			TagStyle{Tags: []*tags.Tag{tags.ProcessingInstruction, tags.String, tags.Inserted}, Color: sage},
			TagStyle{Tags: []*tags.Tag{tags.Invalid}, Color: invalid},
		)
	}(),
})

// Dracula is @uiw/codemirror-theme-dracula 4.25.12.
var Dracula = cmTheme(true, Theme{
	Background:       rgb(0x282a36),
	Foreground:       rgb(0xf8f8f2),
	Selection:        rgba(0xffffff1a),
	GutterBackground: rgb(0x282a36),
	GutterForeground: rgb(0x6d8a88),
	Highlight: NewHighlightStyle(
		TagStyle{Tags: []*tags.Tag{tags.Comment}, Color: rgb(0x6272a4)},
		TagStyle{Tags: []*tags.Tag{tags.String}, Color: rgb(0xf1fa8c)},
		TagStyle{Tags: []*tags.Tag{tags.Atom}, Color: rgb(0xbd93f9)},
		TagStyle{Tags: []*tags.Tag{tags.Meta}, Color: rgb(0xf8f8f2)},
		TagStyle{Tags: []*tags.Tag{tags.Keyword, tags.Operator, tags.TagName}, Color: rgb(0xff79c6)},
		TagStyle{Tags: []*tags.Tag{tags.Function(tags.PropertyName), tags.PropertyName}, Color: rgb(0x66d9ef)},
		TagStyle{Tags: []*tags.Tag{tags.Definition(tags.VariableName), tags.Function(tags.VariableName), tags.ClassName, tags.AttributeName}, Color: rgb(0x50fa7b)},
		TagStyle{Tags: []*tags.Tag{tags.Atom}, Color: rgb(0xbd93f9)},
	),
})

// GitHubLight is githubLight from @uiw/codemirror-theme-github 4.25.12. Its
// deleted text's background is not drawn, nor would CodeMirror draw it: the
// theme writes it without a #.
var GitHubLight = cmTheme(false, Theme{
	Background:       rgb(0xffffff),
	Foreground:       rgb(0x24292e),
	Selection:        rgb(0xbbdfff),
	GutterBackground: rgb(0xffffff),
	GutterForeground: rgb(0x6e7781),
	Highlight: NewHighlightStyle(
		TagStyle{Tags: []*tags.Tag{tags.Standard(tags.TagName), tags.TagName}, Color: rgb(0x116329)},
		TagStyle{Tags: []*tags.Tag{tags.Comment, tags.Bracket}, Color: rgb(0x6a737d)},
		TagStyle{Tags: []*tags.Tag{tags.ClassName, tags.PropertyName}, Color: rgb(0x6f42c1)},
		TagStyle{Tags: []*tags.Tag{tags.VariableName, tags.AttributeName, tags.Number, tags.Operator}, Color: rgb(0x005cc5)},
		TagStyle{Tags: []*tags.Tag{tags.Keyword, tags.TypeName, tags.TypeOperator}, Color: rgb(0xd73a49)},
		TagStyle{Tags: []*tags.Tag{tags.String, tags.Meta, tags.Regexp}, Color: rgb(0x032f62)},
		TagStyle{Tags: []*tags.Tag{tags.Name, tags.Quote}, Color: rgb(0x22863a)},
		TagStyle{Tags: []*tags.Tag{tags.Heading, tags.Strong}, Color: rgb(0x24292e), Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Emphasis}, Color: rgb(0x24292e), Italic: true},
		TagStyle{Tags: []*tags.Tag{tags.Deleted}, Color: rgb(0xb31d28)},
		TagStyle{Tags: []*tags.Tag{tags.Atom, tags.Bool, tags.Special(tags.VariableName)}, Color: rgb(0xe36209)},
		TagStyle{Tags: []*tags.Tag{tags.URL, tags.Escape, tags.Regexp, tags.Link}, Color: rgb(0x032f62)},
		TagStyle{Tags: []*tags.Tag{tags.Link}, Underline: true},
		TagStyle{Tags: []*tags.Tag{tags.Strikethrough}, Strikethrough: true},
		TagStyle{Tags: []*tags.Tag{tags.Invalid}, Color: rgb(0xcb2431)},
	),
})

// GitHubDark is githubDark from @uiw/codemirror-theme-github 4.25.12. It
// sets no gutter colors, so its gutter is @codemirror/view's dark one.
var GitHubDark = cmTheme(true, Theme{
	Background:       rgb(0x0d1117),
	Foreground:       rgb(0xc9d1d9),
	Selection:        rgb(0x003d73),
	GutterBackground: rgb(0x333338),
	GutterForeground: rgb(0xcccccc),
	Highlight: NewHighlightStyle(
		TagStyle{Tags: []*tags.Tag{tags.Standard(tags.TagName), tags.TagName}, Color: rgb(0x7ee787)},
		TagStyle{Tags: []*tags.Tag{tags.Comment, tags.Bracket}, Color: rgb(0x8b949e)},
		TagStyle{Tags: []*tags.Tag{tags.ClassName, tags.PropertyName}, Color: rgb(0xd2a8ff)},
		TagStyle{Tags: []*tags.Tag{tags.VariableName, tags.AttributeName, tags.Number, tags.Operator}, Color: rgb(0x79c0ff)},
		TagStyle{Tags: []*tags.Tag{tags.Keyword, tags.TypeName, tags.TypeOperator}, Color: rgb(0xff7b72)},
		TagStyle{Tags: []*tags.Tag{tags.String, tags.Meta, tags.Regexp}, Color: rgb(0xa5d6ff)},
		TagStyle{Tags: []*tags.Tag{tags.Name, tags.Quote}, Color: rgb(0x7ee787)},
		TagStyle{Tags: []*tags.Tag{tags.Heading, tags.Strong}, Color: rgb(0xd2a8ff), Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Emphasis}, Color: rgb(0xd2a8ff), Italic: true},
		TagStyle{Tags: []*tags.Tag{tags.Deleted}, Color: rgb(0xffdcd7)},
		TagStyle{Tags: []*tags.Tag{tags.Atom, tags.Bool, tags.Special(tags.VariableName)}, Color: rgb(0xffab70)},
		TagStyle{Tags: []*tags.Tag{tags.Link}, Underline: true},
		TagStyle{Tags: []*tags.Tag{tags.Strikethrough}, Strikethrough: true},
		TagStyle{Tags: []*tags.Tag{tags.Invalid}, Color: rgb(0xf97583)},
	),
})

// Nord is @uiw/codemirror-theme-nord 4.25.12. The dotted border under
// invalid code is not drawn.
var Nord = cmTheme(true, Theme{
	Background:       rgb(0x2e3440),
	Foreground:       rgb(0xffffff),
	Selection:        rgba(0x00000073),
	GutterBackground: rgb(0x2e3440),
	GutterForeground: rgb(0x4c566a),
	Highlight: NewHighlightStyle(
		TagStyle{Tags: []*tags.Tag{tags.Keyword}, Color: rgb(0x5e81ac)},
		TagStyle{Tags: []*tags.Tag{tags.Name, tags.Deleted, tags.Character, tags.PropertyName, tags.MacroName}, Color: rgb(0x88c0d0)},
		TagStyle{Tags: []*tags.Tag{tags.VariableName}, Color: rgb(0x8fbcbb)},
		TagStyle{Tags: []*tags.Tag{tags.Function(tags.VariableName)}, Color: rgb(0x8fbcbb)},
		TagStyle{Tags: []*tags.Tag{tags.LabelName}, Color: rgb(0x81a1c1)},
		TagStyle{Tags: []*tags.Tag{tags.Color, tags.Constant(tags.Name), tags.Standard(tags.Name)}, Color: rgb(0x5e81ac)},
		TagStyle{Tags: []*tags.Tag{tags.Definition(tags.Name), tags.Separator}, Color: rgb(0xa3be8c)},
		TagStyle{Tags: []*tags.Tag{tags.Brace}, Color: rgb(0x8fbcbb)},
		TagStyle{Tags: []*tags.Tag{tags.Annotation}, Color: rgb(0xd30102)},
		TagStyle{Tags: []*tags.Tag{tags.Number, tags.Changed, tags.Annotation, tags.Modifier, tags.Self, tags.Namespace}, Color: rgb(0xb48ead)},
		TagStyle{Tags: []*tags.Tag{tags.TypeName, tags.ClassName}, Color: rgb(0xebcb8b)},
		TagStyle{Tags: []*tags.Tag{tags.Operator, tags.OperatorKeyword}, Color: rgb(0xa3be8c)},
		TagStyle{Tags: []*tags.Tag{tags.TagName}, Color: rgb(0xb48ead)},
		TagStyle{Tags: []*tags.Tag{tags.SquareBracket}, Color: rgb(0xbf616a)},
		TagStyle{Tags: []*tags.Tag{tags.AngleBracket}, Color: rgb(0xd08770)},
		TagStyle{Tags: []*tags.Tag{tags.AttributeName}, Color: rgb(0xebcb8b)},
		TagStyle{Tags: []*tags.Tag{tags.Regexp}, Color: rgb(0x5e81ac)},
		TagStyle{Tags: []*tags.Tag{tags.Quote}, Color: rgb(0xb48ead)},
		TagStyle{Tags: []*tags.Tag{tags.String}, Color: rgb(0xa3be8c)},
		TagStyle{Tags: []*tags.Tag{tags.Link}, Color: rgb(0xa3be8c), Underline: true},
		TagStyle{Tags: []*tags.Tag{tags.URL, tags.Escape, tags.Special(tags.String)}, Color: rgb(0x8fbcbb)},
		TagStyle{Tags: []*tags.Tag{tags.Meta}, Color: rgb(0x88c0d0)},
		TagStyle{Tags: []*tags.Tag{tags.Monospace}, Color: rgb(0xd8dee9), Italic: true},
		TagStyle{Tags: []*tags.Tag{tags.Comment}, Color: rgb(0x4c566a), Italic: true},
		TagStyle{Tags: []*tags.Tag{tags.Strong}, Color: rgb(0x5e81ac), Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Emphasis}, Color: rgb(0x5e81ac), Italic: true},
		TagStyle{Tags: []*tags.Tag{tags.Strikethrough}, Strikethrough: true},
		TagStyle{Tags: []*tags.Tag{tags.Heading}, Color: rgb(0x5e81ac), Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Special(tags.Heading1)}, Color: rgb(0x5e81ac), Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Heading1}, Color: rgb(0x5e81ac), Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Heading2, tags.Heading3, tags.Heading4}, Color: rgb(0x5e81ac), Bold: true},
		TagStyle{Tags: []*tags.Tag{tags.Heading5, tags.Heading6}, Color: rgb(0x5e81ac)},
		TagStyle{Tags: []*tags.Tag{tags.Atom, tags.Bool, tags.Special(tags.VariableName)}, Color: rgb(0xd08770)},
		TagStyle{Tags: []*tags.Tag{tags.ProcessingInstruction, tags.Inserted}, Color: rgb(0x8fbcbb)},
		TagStyle{Tags: []*tags.Tag{tags.ContentSeparator}, Color: rgb(0xebcb8b)},
		TagStyle{Tags: []*tags.Tag{tags.Invalid}, Color: rgb(0x434c5e)},
	),
})

// The colors of @codemirror/search 6.7.2's base theme, for a theme that sets
// none.
var (
	cmSearchLight, cmSearchCurrentLight = rgba(0xffff0054), rgba(0xff6a0054)
	cmSearchDark, cmSearchCurrentDark   = rgba(0x00ffff8a), rgba(0xff00ff8a)
)

// cmTheme completes a ported theme with what CodeMirror has no setting for:
// the divider of a Split and the diff colors. The diff colors are DarkTheme's
// or LightTheme's, translucent so they sit on any background; the divider and
// the affordances' hover are white or black over it, as DarkTheme's are.
func cmTheme(dark bool, t Theme) Theme {
	base, overlay := LightTheme, rgb(0x000000)
	searchMatch, searchCurrent := cmSearchLight, cmSearchCurrentLight
	if dark {
		base, overlay = DarkTheme, rgb(0xffffff)
		searchMatch, searchCurrent = cmSearchDark, cmSearchCurrentDark
	}
	if t.SearchMatch == (color.NRGBA{}) {
		t.SearchMatch, t.SearchCurrent = searchMatch, searchCurrent
	}
	t.Border = over(percent(overlay, 10), t.Background)
	t.Diff = base.Diff
	t.Diff.Separator = percent(overlay, 10)
	t.Diff.AffordanceHover = percent(overlay, 10)
	return t
}

// rgba is a CSS #rrggbbaa color.
func rgba(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}
}
