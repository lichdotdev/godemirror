//go:build !godemirror_nochroma

package godemirror

import (
	"testing"

	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/lichdotdev/godemirror/tags"
)

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
func BenchmarkHighlightJavaChroma(b *testing.B) {
	text := normalize(bigJava(b))
	lexer := lexers.Get("java")
	for b.Loop() {
		if _, err := highlightChroma(lexer, text); err != nil {
			b.Fatal(err)
		}
	}
}

// bigHTML is an HTML page of about benchJavaLines lines, its sections each
// nesting a script and a style sheet, and attributes CSS and JavaScript
// parse, as @codemirror/lang-html nests them.
