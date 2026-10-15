package tags_test

import (
	"slices"
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
