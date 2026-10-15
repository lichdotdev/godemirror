package godemirror

import (
	"fmt"
	"image"
	"image/color"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/tags"
)

// incrementalSamples are, by file name, a valid piece of code in each
// language Lezer parses, repeated with its names numbered into a document
// long enough for a reparse to reuse nodes (Lezer reuses none in a text
// under 4096 code units).
var incrementalSamples = map[string]string{
	"a.java": "class A%[1]d {\n\tint f(int x) { return x + 1; } // note\n\tString s = \"str %[1]d\";\n\t/* block */ void g() { if (s != null) { f(2); } }\n}\n",
	"a.go":   "func f%[1]d(x int) int { return x + 1 } // note\n\nvar s%[1]d = \"str\"\n\ntype T%[1]d struct {\n\tA int `json:\"a\"`\n}\n",
	"a.py":   "def f%[1]d(x):\n    if x:\n        return x + 1  # note\n    return 'str'\n\nclass C%[1]d:\n    s = [1, 2, 3]\n",
	"a.rs":   "fn f%[1]d(x: i32) -> i32 { x + 1 } // note\nconst S%[1]d: &str = \"str\";\nstruct T%[1]d { a: Vec<u8> }\n",
	"a.css":  ".a%[1]d > p { color: red; margin: 0 1px; } /* note */\n@media screen { #b%[1]d { width: calc(100%% - 2px); } }\n",
	"a.js":   "function f%[1]d(x) { return x + 1; } // note\nconst s%[1]d = `str ${x}`;\nclass C%[1]d { m() { return [1, 2]; } }\n",
	"a.ts":   "function f%[1]d(x: number): number { return x + 1; } // note\ninterface I%[1]d { a: string }\nlet v%[1]d: I%[1]d = { a: \"str\" };\n",
	"a.html": "<div class=\"a%[1]d\" style=\"color: red\">\n<p>text &amp; more</p>\n<script>let x%[1]d = f(1); // note\n</script>\n<style>.a { margin: 0; }</style>\n</div>\n",
}

func sampleText(filename string) string {
	var b strings.Builder
	if filename == "a.go" {
		b.WriteString("package a\n\n")
	}
	for i := 0; b.Len() < 9000; i++ {
		fmt.Fprintf(&b, incrementalSamples[filename], i)
	}
	return b.String()
}

// TestIncrementalHighlightingIsAFreshOnes edits valid code of every Lezer
// language at random, through the incremental reparse the view runs, and
// requires the tags a fresh highlight of the same text gives, edit after
// edit. Only edits that leave the code without a syntax error count: in
// broken code Lezer's incremental parse, CodeMirror's too, may recover
// otherwise than a fresh parse does (see docs/ceilings.md).
func TestIncrementalHighlightingIsAFreshOnes(t *testing.T) {
	for filename := range incrementalSamples {
		t.Run(filename, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(1, uint64(len(filename))))
			text := sampleText(filename)
			if hasSyntaxError(t, filename, text) {
				t.Fatal("the sample has a syntax error")
			}
			_, parsed, err := highlightAfter(filename, text, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for edits := 0; edits < 60; {
				next := randomValidEdit(rng, text)
				if hasSyntaxError(t, filename, next) {
					continue
				}
				edits++
				var got *document
				got, parsed, err = highlightAfter(filename, next, nil, parsed)
				if err != nil {
					t.Fatal(err)
				}
				requireSameTags(t, got, next, filename)
				text = next
			}
		})
	}
}

func hasSyntaxError(t *testing.T, filename, text string) bool {
	t.Helper()
	_, parsed, err := highlightAfter(filename, text, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(parsed.tree.String(), "⚠")
}

func requireSameTags(t *testing.T, got *document, text, filename string) {
	t.Helper()
	want, err := highlight(filename, text, nil)
	if err != nil {
		t.Fatal(err)
	}
	for line := range want.lines {
		for cell := range want.tags[line] {
			if tagNames(got.tags[line][cell]) != tagNames(want.tags[line][cell]) {
				t.Fatalf("line %d cell %d is %s, a fresh highlight %s, in:\n%s", line, cell,
					tagNames(got.tags[line][cell]), tagNames(want.tags[line][cell]), string(want.source[line]))
			}
		}
	}
}

// tagNames names a cell's tags: TagSets from separate highlights are not
// the same pointers.
func tagNames(set *lezer.TagSet) string {
	if set == nil {
		return ""
	}
	names := make([]string, len(set.Tags))
	for i, tag := range set.Tags {
		names[i] = tag.String()
	}
	return strings.Join(names, " ")
}

// randomValidEdit is text with one small edit of a kind that often keeps
// code valid: a letter typed or deleted inside a word, a space doubled, a
// line duplicated or deleted.
func randomValidEdit(rng *rand.Rand, text string) string {
	r := []rune(text)
	switch rng.IntN(5) {
	case 0:
		if i := 1 + rng.IntN(len(r)-1); unicode.IsLetter(r[i-1]) {
			return string(slices.Insert(r, i, rune('a'+rng.IntN(26))))
		}
	case 1:
		if i := 1 + rng.IntN(len(r)-2); unicode.IsLetter(r[i-1]) && unicode.IsLetter(r[i]) && unicode.IsLetter(r[i+1]) {
			return string(slices.Delete(r, i, i+1))
		}
	case 2:
		if i := 1 + rng.IntN(len(r)-1); r[i-1] == ' ' {
			return string(slices.Insert(r, i, ' '))
		}
	default:
		lines := strings.SplitAfter(text, "\n")
		i := rng.IntN(len(lines) - 1)
		if rng.IntN(2) == 0 {
			return strings.Join(slices.Insert(lines, i, lines[i]), "")
		}
		return strings.Join(slices.Delete(lines, i, i+1), "")
	}
	return text
}

// TestUpdateContentRecolorsAnEditedLine edits a big Java file through
// UpdateContent, commenting a line out and back in, and requires the line
// drawn in the comment's color and then in its code's again, with every
// cell tagged as a fresh SetContent of the same text tags it.
func TestUpdateContentRecolorsAnEditedLine(t *testing.T) {
	h := newHarness(t, "")
	h.v.Theme = OneDark
	text := sampleText("a.java")
	lines := strings.SplitAfter(text, "\n")
	line := 100
	for !strings.HasPrefix(lines[line], "\tint f(") {
		line++
	}
	h.v.SetContent(Content{Filename: "A.java", Text: text})
	waitForHighlight(h)
	comment := h.v.Theme.Highlight.style(h.v.doc.tags[line][len(h.v.doc.tags[line])-1], h.v.Theme.Foreground).Color
	// Commented, the tab takes cells 0 to 3, "//" 4 and 5, and "int" 6 to 8.
	const intCell = 6
	for _, commented := range []bool{true, false} {
		edited := slices.Clone(lines)
		if commented {
			edited[line] = "\t//" + strings.TrimPrefix(lines[line], "\t")
		}
		next := strings.Join(edited, "")
		h.v.UpdateContent(Content{Filename: "A.java", Text: next})
		waitForHighlight(h)
		requireSameTags(t, h.v.doc, next, "A.java")
		img := h.v.rasterize(h.v.doc.lines[line], h.v.doc.tags[line])
		keyword := image.Rect(intCell*h.v.r.CellW(), 0, (intCell+3)*h.v.r.CellW(), img.Bounds().Dy())
		if commented && onlyTag(t, h.v.doc.tags[line][intCell]) != tags.LineComment {
			t.Fatalf("the commented line's %q is tagged %v", "int", tagNames(h.v.doc.tags[line][intCell]))
		}
		if inkedIn(img, keyword, comment) != commented {
			t.Fatalf("commented %v: the line's start drawn in the comment color is %v", commented, !commented)
		}
	}
}

// inkedIn reports whether a glyph in band of img, a row image composed over
// transparency, is drawn in c. It reads the ink's color from pixels the
// glyph mostly covers, not from a fully covered one: at small sizes a thin
// font's strokes may cover no pixel whole.
func inkedIn(img *image.RGBA, band image.Rectangle, c color.NRGBA) bool {
	const mostlyCovered, tolerance = 0x80, 3
	near := func(premultiplied, alpha, want uint8) bool {
		got := int(premultiplied) * 0xff / int(alpha)
		return got >= int(want)-tolerance && got <= int(want)+tolerance
	}
	for y := band.Min.Y; y < band.Max.Y; y++ {
		for x := band.Min.X; x < band.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.A >= mostlyCovered && near(p.R, p.A, c.R) && near(p.G, p.A, c.G) && near(p.B, p.A, c.B) {
				return true
			}
		}
	}
	return false
}

// TestABurstOfUpdatesEndsOnTheLastContent sends many updates before any is
// highlighted, as an editor does while its user types: the view ends on the
// last one, tagged as a fresh highlight tags it.
func TestABurstOfUpdatesEndsOnTheLastContent(t *testing.T) {
	h := newHarness(t, "")
	text := sampleText("a.java")
	h.v.SetContent(Content{Filename: "A.java", Text: text})
	for i := range 20 {
		text = strings.Replace(text, "return x + 1;", fmt.Sprintf("return x + %d; // typed", i), 1)
		h.v.UpdateContent(Content{Filename: "A.java", Text: text})
	}
	waitForHighlight(h)
	shown := make([]string, h.v.LineCount())
	for i := range shown {
		shown[i] = h.v.LineText(i)
	}
	if strings.Join(shown, "\n") != normalize(text) {
		t.Fatal("the view does not show the last content")
	}
	requireSameTags(t, h.v.doc, text, "A.java")
}
