package tags_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror/tags"
)

func names(t *tags.Tag) []string {
	var out []string
	for s := range t.Set() {
		out = append(out, s.String())
	}
	return out
}

func TestSetFallsBackThroughParents(t *testing.T) {
	got := names(tags.AttributeName)
	want := []string{"attributeName", "propertyName", "name"}
	if !slices.Equal(got, want) {
		t.Fatalf("set = %v, want %v", got, want)
	}
}

// The order is @lezer/highlight 1.2.5's, printed from
// tags.definition(tags.function(tags.variableName)).set.
func TestModifiedSetMatchesLezer(t *testing.T) {
	got := names(tags.Definition(tags.Function(tags.VariableName)))
	want := []string{
		"function(definition(variableName))",
		"definition(variableName)",
		"function(variableName)",
		"variableName",
		"function(definition(name))",
		"definition(name)",
		"function(name)",
		"name",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("set =\n%v\nwant\n%v", got, want)
	}
}

func TestModifiersAreInterned(t *testing.T) {
	a := tags.Definition(tags.Function(tags.VariableName))
	b := tags.Function(tags.Definition(tags.VariableName))
	if a != b {
		t.Fatal("the same modifiers in another order gave another tag")
	}
	if tags.Special(tags.Special(tags.String)) != tags.Special(tags.String) {
		t.Fatal("a modifier applied twice gave another tag")
	}
}

// Parse reads back what String prints, modifiers included, and gives the
// same tag as building it in Go.
func TestParseReadsWhatStringPrints(t *testing.T) {
	cases := []*tags.Tag{
		tags.Keyword,
		tags.LineComment,
		tags.Function(tags.VariableName),
		tags.Definition(tags.Function(tags.VariableName)),
		tags.Special(tags.String),
		tags.Constant(tags.Standard(tags.Name)),
	}
	for _, want := range cases {
		got, err := tags.Parse(want.String())
		if err != nil {
			t.Fatalf("Parse(%q): %v", want, err)
		}
		if got != want {
			t.Errorf("Parse(%q) = %q, not the same tag", want, got)
		}
	}
}

func TestParseRefusesWhatIsNoTag(t *testing.T) {
	cases := []struct{ expr, reason string }{
		{"keywords", `"keywords" is not a highlighting tag: want a standard tag name`},
		{"", `"" is not a highlighting tag`},
		{" keyword", `" keyword" is not a highlighting tag`},
		{"bold(keyword)", `"bold" in "bold(keyword)" is not a tag modifier: want one of definition, constant, function, standard, local, special`},
		{"function(keyword", `"function(keyword" opens a parenthesis it never closes`},
		{"function(definition(keyword)", `opens a parenthesis it never closes`},
		{"keyword)", `"keyword)" closes a parenthesis it never opened`},
		{"function(keyword))", `has ")" after the parenthesis that closes it: want nothing there`},
		{"function(keyword)x", `has "x" after the parenthesis that closes it`},
		{"function()", `"" is not a highlighting tag`},
	}
	for _, tc := range cases {
		got, err := tags.Parse(tc.expr)
		if err == nil || !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("Parse(%q) = %v, %v; want an error containing %s", tc.expr, got, err, tc.reason)
		}
	}
}

// Modifiers name one tag in any order, as applying them in Go does.
func TestParseTakesModifiersInAnyOrder(t *testing.T) {
	a, err := tags.Parse("definition(function(variableName))")
	if err != nil {
		t.Fatal(err)
	}
	b, err := tags.Parse("function(definition(variableName))")
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a != tags.Definition(tags.Function(tags.VariableName)) {
		t.Errorf("Parse gave %q and %q, want one tag", a, b)
	}
}
