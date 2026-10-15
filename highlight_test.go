package godemirror

import (
	"testing"

	"github.com/lichdotdev/godemirror/tags"
)

func TestHighlightCutsLinesAndExpandsTabsToStops(t *testing.T) {
	d, err := highlight("notes.unknown-ext", "a\tb\n\tc\r\n\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a   b", "    c", ""}
	if len(d.lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(d.lines), len(want))
	}
	for i := range want {
		if string(d.lines[i]) != want[i] {
			t.Fatalf("line %d = %q, want %q", i, string(d.lines[i]), want[i])
		}
		if len(d.tags[i]) != len(d.lines[i]) {
			t.Fatalf("line %d has %d tags for %d cells", i, len(d.tags[i]), len(d.lines[i]))
		}
	}
}

// The tags are the ones @lezer/go gives the same code, but for main: lezer
// tags a declared function definition(function(variableName)), and chroma
// cannot tell a declaration from a call.
func TestHighlightTagsGoCode(t *testing.T) {
	d, err := highlight("main.go", "func main() {\n\t// hi\n\treturn \"x\", 42, true, nil\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(d.lines))
	}
	tagAt := func(line int, word string) *tags.Tag {
		text := string(d.lines[line])
		for i := range len(text) - len(word) + 1 {
			if text[i:i+len(word)] == word {
				return d.tags[line][i]
			}
		}
		t.Fatalf("%q not on line %d (%q)", word, line, text)
		return nil
	}
	cases := []struct {
		line int
		word string
		want *tags.Tag
	}{
		{0, "func", tags.DefinitionKeyword},
		{0, "main", tags.Function(tags.VariableName)},
		{0, "(", tags.Paren},
		{0, ")", tags.Paren},
		{0, "{", tags.Brace},
		{1, "// hi", tags.LineComment},
		{2, "return", tags.Keyword},
		{2, `"x"`, tags.String},
		{2, ",", tags.Separator},
		{2, "42", tags.Integer},
		{2, "true", tags.Bool},
		{2, "nil", tags.Null},
		{3, "}", tags.Brace},
	}
	for _, c := range cases {
		if got := tagAt(c.line, c.word); got != c.want {
			t.Errorf("%q on line %d: tag %v, want %v", c.word, c.line, got, c.want)
		}
	}
}

func TestSourceTextGivesTabsBack(t *testing.T) {
	d := plain("a\tb\tc")
	// cells: a=0, tab=1..3, b=4, tab=5..7, c=8
	cases := []struct {
		from, to int
		want     string
	}{
		{0, 9, "a\tb\tc"},
		{0, 1, "a"},
		{1, 5, "\tb"},
		{2, 5, "b"},
		{4, 9, "b\tc"},
		{9, 9, ""},
	}
	for _, c := range cases {
		if got := d.sourceText(0, c.from, c.to); got != c.want {
			t.Errorf("sourceText(%d, %d) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}
