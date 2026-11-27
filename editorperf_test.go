package godemirror

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestAnEditRenumbersTheRowImagesOfTheLinesAfterIt(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "aaa\nbb\nc"})
	waitForHighlight(h)
	h.frame()
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(0, "zzzz\n")}})
	h.frame()
	cw := h.v.r.CellW()
	for line, cells := range []int{4, 3, 2, 1} {
		if got := h.drawnRow(line).size.X; got != cells*cw {
			t.Fatalf("line %d drew a row %dpx wide, want %d cells, %dpx", line+1, got, cells, cells*cw)
		}
	}
}

func TestAnEditedDocumentIsTheDocumentBuiltAnew(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	pieces := []string{"a", "word ", "\t", "世", "\n", "\n", "é", "  "}
	text := func(n int) string {
		var b strings.Builder
		for range n {
			b.WriteString(pieces[r.IntN(len(pieces))])
		}
		return b.String()
	}
	h := newEditorHarness(t, EditorStateConfig{Doc: text(60)})
	h.ed.Wrap = true
	h.size.X = 120
	h.frame()
	for step := range 200 {
		doc := h.ed.State().Doc()
		from := r.IntN(len(doc) + 1)
		for from < len(doc) && doc[from]&0xc0 == 0x80 {
			from++
		}
		to := min(from+r.IntN(6), len(doc))
		for to < len(doc) && doc[to]&0xc0 == 0x80 {
			to++
		}
		dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{{From: from, To: to, Insert: text(r.IntN(4))}}})
		h.frame()
		want := plain(h.ed.State().Doc() + "\n")
		got := h.v.doc
		for i := range want.lines {
			if !slices.Equal(got.lines[i], want.lines[i]) || !slices.Equal(got.source[i], want.source[i]) ||
				!slices.Equal(got.cellOf[i], want.cellOf[i]) || !slices.Equal(h.v.wraps[i], wrapLine(want.lines[i], h.v.wrapCols)) {
				t.Fatalf("step %d: line %d is %q, built anew %q", step, i, string(got.source[i]), string(want.source[i]))
			}
		}
		if len(got.lines) != len(want.lines) {
			t.Fatalf("step %d: %d lines, built anew %d", step, len(got.lines), len(want.lines))
		}
	}
}

func TestRenumberRowsKeepsMovesAndDrops(t *testing.T) {
	h := newHarness(t, "")
	before, edited, after := &rowImage{}, &rowImage{}, &rowImage{}
	h.v.rows = map[rowKey]*rowImage{{line: 0}: before, {line: 2}: edited, {line: 5, segment: 1}: after}
	h.v.gutter = map[gutterKey]*rowImage{{line: 0, text: "1"}: {}}
	// Lines [2, 4) became three lines: one more.
	h.v.renumberRows(2, 4, 1)
	want := map[rowKey]*rowImage{{line: 0}: before, {line: 6, segment: 1}: after}
	if len(h.v.rows) != len(want) || h.v.rows[rowKey{line: 0}] != before || h.v.rows[rowKey{line: 6, segment: 1}] != after {
		t.Fatalf("rows %v, want line 0 kept, line 2 dropped and line 5 moved to 6", h.v.rows)
	}
	if len(h.v.gutter) != 0 {
		t.Fatal("the gutter's images were kept")
	}
}
