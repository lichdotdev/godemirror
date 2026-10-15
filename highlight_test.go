package godemirror

import (
	"slices"
	"testing"

	"github.com/alecthomas/chroma/v2/lexers"

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

// chroma is the fallback for languages with no Lezer port; Go has one, but
// its lexer is a known one to check tagmap against. The tags are the ones
// @lezer/go gives the same code, but for main, return and 42: lezer tags a
// declared function function(definition(variableName)), return a
// controlKeyword and 42 a number, and chroma cannot tell those apart.
func TestChromaTagsGoCode(t *testing.T) {
	d, err := highlightChroma(lexers.Get("go"), normalize("func main() {\n\t// hi\n\treturn \"x\", 42, true, nil\n}\n"))
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
				return onlyTag(t, d.tags[line][i])
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

// Java goes through the port of @lezer/java, which tells a call from a
// declaration where chroma cannot. Lezer counts in UTF-16 units, so the
// tags after a character outside the BMP (two units, two cells here) and a
// tab must still land on their own cells.
func TestHighlightTagsJavaAsLezerDoes(t *testing.T) {
	d, err := highlight("A.java", "class A {\n\tvoid run() { log(\"😀\", 1); }\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(d.lines))
	}
	cases := []struct {
		line int
		word string
		want *tags.Tag
	}{
		{0, "class", tags.DefinitionKeyword},
		{0, "A", tags.Definition(tags.VariableName)},
		{1, "void", tags.Keyword},
		{1, "run", tags.Definition(tags.VariableName)},
		{1, "log", tags.Function(tags.VariableName)},
		{1, `"😀"`, tags.String},
		{1, "1", tags.Integer},
		{1, ")", tags.Paren},
		{1, ";", tags.Separator},
		{2, "}", tags.Brace},
	}
	for _, c := range cases {
		cells, want := d.lines[c.line], textCells(c.word)
		at := -1
		for i := range len(cells) - len(want) + 1 {
			if slices.Equal(cells[i:i+len(want)], want) {
				at = i
				break
			}
		}
		if at < 0 {
			t.Fatalf("%q not on line %d (%q)", c.word, c.line, string(cells))
		}
		for i := at; i < at+len(want); i++ {
			if got := onlyTag(t, d.tags[c.line][i]); got != c.want {
				t.Errorf("%q on line %d, cell %d: tag %v, want %v", c.word, c.line, i, got, c.want)
			}
		}
	}
}

// Go goes through the port of @lezer/go, so its tags are the grammar's own,
// where chroma's differ.
func TestHighlightTagsGoAsLezerDoes(t *testing.T) {
	d, err := highlight("main.go", "func main() {\n\treturn 42\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		line, cell int
		want       *tags.Tag
	}{
		{0, len("func "), tags.Function(tags.Definition(tags.VariableName))},
		{1, tabSize, tags.ControlKeyword},
		{1, tabSize + len("return "), tags.Number},
	}
	for _, c := range cases {
		if got := onlyTag(t, d.tags[c.line][c.cell]); got != c.want {
			t.Errorf("line %d, cell %d: tag %v, want %v", c.line, c.cell, got, c.want)
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

// Kotlin and shell go through the ports of their @codemirror/legacy-modes
// modes, for every extension and name @codemirror/language-data gives
// them. The 😀 before each checked word is two UTF-16 units and two cells,
// so a tag that lands on its own cells proves the offsets were converted.
func TestHighlightTagsKotlinAndShellAsTheirLegacyModesDo(t *testing.T) {
	kotlinCode := "val 😀 = \"s\"\n\tfun name() = 1 // c\n"
	shellCode := "#!/bin/sh\n😀=$HOME; echo 'x' --all\n"
	cases := []struct {
		filename, text string
		line           int
		word           string
		want           *tags.Tag
	}{
		{"A.kt", kotlinCode, 0, "val", tags.Keyword},
		{"A.KT", kotlinCode, 0, `"s"`, tags.String},
		{"build.gradle.kts", kotlinCode, 1, "name", tags.Definition(tags.VariableName)},
		{"A.kt", kotlinCode, 1, "1", tags.Number},
		{"A.kt", kotlinCode, 1, "// c", tags.Comment},
		{"run.sh", shellCode, 0, "#!/bin/sh", tags.Meta},
		{"run.bash", shellCode, 1, "$HOME", tags.Definition(tags.VariableName)},
		{"run.ksh", shellCode, 1, "echo", tags.Standard(tags.VariableName)},
		{"pkg/PKGBUILD", shellCode, 1, "'x'", tags.String},
		{"PKGBUILD", shellCode, 1, "--all", tags.AttributeName},
	}
	for _, c := range cases {
		d, err := highlight(c.filename, c.text)
		if err != nil {
			t.Fatal(err)
		}
		cells, want := d.lines[c.line], textCells(c.word)
		at := -1
		for i := range len(cells) - len(want) + 1 {
			if slices.Equal(cells[i:i+len(want)], want) {
				at = i
				break
			}
		}
		if at < 0 {
			t.Fatalf("%s: %q not on line %d (%q)", c.filename, c.word, c.line, string(cells))
		}
		for i := at; i < at+len(want); i++ {
			if got := onlyTag(t, d.tags[c.line][i]); got != c.want {
				t.Errorf("%s: %q on line %d, cell %d: tag %v, want %v", c.filename, c.word, c.line, i, got, c.want)
			}
		}
	}
}
