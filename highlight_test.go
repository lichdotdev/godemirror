package godemirror

import (
	"slices"
	"testing"
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
		if len(d.classes[i]) != len(d.lines[i]) {
			t.Fatalf("line %d has %d classes for %d cells", i, len(d.classes[i]), len(d.lines[i]))
		}
	}
}

func TestHighlightClassesGoCode(t *testing.T) {
	d, err := highlight("main.go", "func main() {\n\t// hi\n\treturn \"x\", 42\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(d.lines))
	}
	classAt := func(line int, word string) TokenClass {
		text := string(d.lines[line])
		for i := range len(text) - len(word) + 1 {
			if text[i:i+len(word)] == word {
				return d.classes[line][i]
			}
		}
		t.Fatalf("%q not on line %d (%q)", word, line, text)
		return Plain
	}
	cases := []struct {
		line int
		word string
		want TokenClass
	}{
		{0, "func", Keyword},
		{0, "main", Definition},
		{1, "// hi", Comment},
		{2, "return", Keyword},
		{2, `"x"`, String},
		{2, "42", Literal},
	}
	for _, c := range cases {
		if got := classAt(c.line, c.word); got != c.want {
			t.Errorf("%q on line %d: class %d, want %d", c.word, c.line, got, c.want)
		}
	}
	if !slices.Contains(d.classes[3], Punctuation) {
		t.Errorf("closing brace not punctuation: %v", d.classes[3])
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
