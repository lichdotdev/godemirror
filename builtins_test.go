package godemirror

import (
	"testing"

	"github.com/lichdotdev/godemirror/tags"
)

// textTagCase is a word on a line of a file and the tag it must get.
type textTagCase struct {
	file, text string
	line       int
	word       string
	want       *tags.Tag
}

// requireTags highlights each case's file with styles and requires its word
// to get its tag.
func requireTags(t *testing.T, styles []StyleTags, cases []textTagCase) {
	t.Helper()
	for _, c := range cases {
		h := newHarness(t, "")
		h.v.SetContent(Content{Filename: c.file, Text: c.text, StyleTags: styles})
		waitForHighlight(h)
		if got := tagAt(t, h.v, c.line, c.word); got != c.want {
			t.Errorf("%s: %q on line %d is %v, want %v", c.file, c.word, c.line, got, c.want)
		}
	}
}

// A rule with Names tags only the nodes whose text is one of them, and
// every other node keeps the grammar's tag, even when the rule is no
// deeper than the grammar's own.
func TestStyleRuleNamesMatchANodesText(t *testing.T) {
	style := StyleTags{Language: "go", Rules: []StyleRule{
		{Selector: "VariableName", Names: []string{"alpha"}, Tags: []*tags.Tag{tags.Constant(tags.VariableName)}},
		{Selector: "CallExpr/VariableName", Names: []string{"beta"}, Tags: []*tags.Tag{tags.Keyword}},
	}}
	text := "package p\n\nvar _ = []int{alpha, gamma, beta(delta), omega(alpha)}\n"
	requireTags(t, []StyleTags{style}, []textTagCase{
		{"p.go", text, 2, "alpha", tags.Constant(tags.VariableName)},
		{"p.go", text, 2, "gamma", tags.VariableName},
		{"p.go", text, 2, "beta", tags.Keyword},
		{"p.go", text, 2, "delta", tags.VariableName},
		{"p.go", text, 2, "omega", tags.Function(tags.VariableName)},
	})
}

func TestBuiltinTagsTagTheLanguagesBuiltins(t *testing.T) {
	stdFunc := tags.Function(tags.Standard(tags.VariableName))
	stdType := tags.Standard(tags.TypeName)
	stdVar := tags.Standard(tags.VariableName)
	goText := "package p\n\nvar e error = f(int(x), len(s), lens(s), iota)\n"
	requireTags(t, []StyleTags{GoBuiltinTags()}, []textTagCase{
		{"p.go", goText, 2, "error", stdType},
		{"p.go", goText, 2, "int", stdType},
		{"p.go", goText, 2, "len", stdFunc},
		{"p.go", goText, 2, "lens", tags.Function(tags.VariableName)},
		{"p.go", goText, 2, "iota", stdVar},
	})
	pyText := "x = len(str)\nlength(x)\n"
	requireTags(t, []StyleTags{PythonBuiltinTags()}, []textTagCase{
		{"p.py", pyText, 0, "len", stdFunc},
		{"p.py", pyText, 0, "str", stdVar},
		{"p.py", pyText, 1, "length", tags.Function(tags.VariableName)},
	})
	jsText := "new Map(parseInt(x), undefined, Math.max, Mapper)\n"
	requireTags(t, []StyleTags{JavaScriptBuiltinTags()}, []textTagCase{
		{"p.js", jsText, 0, "Map", tags.Standard(tags.ClassName)},
		{"p.js", jsText, 0, "parseInt", stdFunc},
		{"p.js", jsText, 0, "undefined", stdVar},
		{"p.js", jsText, 0, "Math", stdVar},
		{"p.js", jsText, 0, "Mapper", tags.VariableName},
	})
	tsText := "let a: string = b as Thing\n"
	requireTags(t, []StyleTags{JavaScriptBuiltinTags()}, []textTagCase{
		{"p.ts", tsText, 0, "string", stdType},
		{"p.ts", tsText, 0, "Thing", tags.TypeName},
	})
	rsText := "fn f(x: u8) -> Vec<usize> { x as Usize }\n"
	requireTags(t, []StyleTags{RustBuiltinTags()}, []textTagCase{
		{"p.rs", rsText, 0, "u8", stdType},
		{"p.rs", rsText, 0, "usize", stdType},
		{"p.rs", rsText, 0, "Vec", tags.TypeName},
		{"p.rs", rsText, 0, "Usize", tags.TypeName},
	})
}

// Builtin and Richer sets for one grammar combine: each tags what it names.
func TestBuiltinTagsCombineWithRicherTags(t *testing.T) {
	text := "package p\n\nfunc f(alpha int) []int { return make([]int, len(alpha)) }\n"
	requireTags(t, []StyleTags{RicherGoTags(), GoBuiltinTags()}, []textTagCase{
		{"p.go", text, 2, "alpha", tags.Definition(tags.Local(tags.VariableName))},
		{"p.go", text, 2, "int", tags.Standard(tags.TypeName)},
		{"p.go", text, 2, "make", tags.Function(tags.Standard(tags.VariableName))},
		{"p.go", text, 2, "len", tags.Function(tags.Standard(tags.VariableName))},
	})
}

// A rule with Names is tried before a rule as deep without, whichever the
// set lists first.
func TestStyleRuleNamesGoBeforeAPlainRuleAsDeep(t *testing.T) {
	style := StyleTags{Language: "go", Rules: []StyleRule{
		{Selector: "VariableName", Tags: []*tags.Tag{tags.Name}},
		{Selector: "VariableName", Names: []string{"alpha"}, Tags: []*tags.Tag{tags.Constant(tags.VariableName)}},
	}}
	text := "package p\n\nvar _ = []int{alpha, gamma}\n"
	requireTags(t, []StyleTags{style}, []textTagCase{
		{"p.go", text, 2, "alpha", tags.Constant(tags.VariableName)},
		{"p.go", text, 2, "gamma", tags.Name},
	})
}
