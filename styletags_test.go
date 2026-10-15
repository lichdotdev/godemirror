package godemirror

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
	"github.com/lichdotdev/godemirror/tags"
)

// TestRicherJavaTagsMatchLezer requires of RicherJavaTags the trees and
// spans the real @lezer/java gives configured with the same styleTags,
// internal/lezer/java/richer.mjs, as golden.mjs wrote them down.
func TestRicherJavaTagsMatchLezer(t *testing.T) {
	p, err := lezerParser("Richer.java", []StyleTags{RicherJavaTags()})
	if err != nil {
		t.Fatal(err)
	}
	lezertest.MatchGoldens(t, p, "internal/lezer/java/testdata/richer")
}

const enumSource = "enum Kind {\n\tOUTFLOW(\"B\"), INFLOW;\n\tprivate final String value;\n}\n"

// tagAt is the one tag of the cell where word starts on line.
func tagAt(t *testing.T, v *View, line int, word string) *tags.Tag {
	t.Helper()
	i := strings.Index(string(v.doc.lines[line]), word)
	if i < 0 {
		t.Fatalf("%q is not on line %d", word, line)
	}
	return onlyTag(t, v.doc.tags[line][i])
}

func TestContentStyleTagsRetagAView(t *testing.T) {
	h := newHarness(t, "")
	h.v.SetContent(Content{Filename: "Kind.java", Text: enumSource})
	waitForHighlight(h)
	if got, want := tagAt(t, h.v, 1, "OUTFLOW"), tags.Definition(tags.VariableName); got != want {
		t.Fatalf("without StyleTags, OUTFLOW is %v, want @lezer/java's %v", got, want)
	}
	h.v.UpdateContent(Content{Filename: "Kind.java", Text: enumSource, StyleTags: []StyleTags{RicherJavaTags()}})
	waitForHighlight(h)
	if got, want := tagAt(t, h.v, 1, "OUTFLOW"), tags.Constant(tags.VariableName); got != want {
		t.Fatalf("with RicherJavaTags, OUTFLOW is %v, want %v", got, want)
	}
	if got, want := tagAt(t, h.v, 2, "value"), tags.Definition(tags.PropertyName); got != want {
		t.Fatalf("with RicherJavaTags, the field is %v, want %v", got, want)
	}
}

func TestDiffOptionsStyleTagsRetagBothSides(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{StyleTags: []StyleTags{RicherJavaTags()}})
	d.SetDocs("Kind.java", enumSource, strings.Replace(enumSource, "INFLOW", "INFLOW, NONE", 1))
	waitForHighlight(h)
	if got, want := tagAt(t, h.v, 1, "OUTFLOW"), tags.Constant(tags.VariableName); got != want {
		t.Fatalf("OUTFLOW is %v, want %v", got, want)
	}
}

// StyleTags for a rule that cannot apply fails the Layout that would show
// its highlighting, rather than leaving the file colored as if it were not
// there.
func TestStyleTagsAMistakeFailsTheLayout(t *testing.T) {
	cases := map[string]StyleTags{
		`want a Lezer grammar, one of css, go, html, java, javascript, python, rust`: {Language: "kotlin"},
		`invalid path "Annotation/"`: {Language: "java", Rules: []StyleRule{{Selector: "Annotation/", Tags: []*tags.Tag{tags.Meta}}}},
	}
	for want, style := range cases {
		v, err := New("monospace", 12)
		if err != nil {
			t.Fatal(err)
		}
		v.SetContent(Content{Filename: "Kind.java", Text: enumSource, StyleTags: []StyleTags{style}})
		if err := layoutUntilHighlighted(v); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Layout with StyleTags %+v = %v, want an error containing %q", style, err, want)
		}
	}
}

// layoutUntilHighlighted lays v out until its highlighting lands, and is
// the first error Layout returns.
func layoutUntilHighlighted(v *View) error {
	var ops op.Ops
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Now: time.Now(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 200))}
		if _, err := v.Layout(gtx); err != nil || v.pending == nil {
			return err
		}
	}
	return nil
}
