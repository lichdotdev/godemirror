package godemirror

import "testing"

func TestAnEditedLineKeepsItsColorsUntilHighlightingLands(t *testing.T) {
	doc := "package main\n\nfunc main() {}\n"
	h, _ := focusedEditor(t, EditorStateConfig{Filename: "main.go", Doc: doc}, SingleSelection(22, 22))
	h.until("highlighted", func() bool { return h.v.doc.tags[2][0] != nil })
	// Typing inside "main" of line 3, the frame that shows it comes before
	// the new highlighting: the line keeps the colors it had.
	h.typeText("x")
	tags := h.v.doc.tags[2]
	if string(h.v.doc.lines[2]) != "func maixn() {}" {
		t.Fatalf("line %q", string(h.v.doc.lines[2]))
	}
	if tags[0] == nil || tags[0] != tags[3] {
		t.Fatalf("func lost its color: %v", tags[:4])
	}
	if tags[8] != tags[7] || tags[8] != tags[9] {
		t.Fatalf("the x typed inside main does not take main's color: %v", tags[5:11])
	}
}
