package godemirror

import (
	"testing"

	"gioui.org/io/key"
)

func closingEditor(t *testing.T, doc string, sel EditorSelection) *harness {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: doc}, sel)
	h.ed.CloseBrackets = true
	return h
}

func (h *harness) wantDoc(doc string, cursor int) {
	h.t.Helper()
	if got := h.ed.State().Doc(); got != doc {
		h.t.Fatalf("doc %q, want %q", got, doc)
	}
	if a, hd := h.mainRange(); a != cursor || hd != cursor {
		h.t.Fatalf("selection %d-%d, want a cursor at %d", a, hd, cursor)
	}
}

func TestAnOpeningBracketTypedInsertsItsPair(t *testing.T) {
	h := closingEditor(t, "f", SingleSelection(1, 1))
	h.typeText("(")
	h.wantDoc("f()", 2)
	h.typeText("x")
	h.typeText(")")
	h.wantDoc("f(x)", 4)
}

func TestAQuoteTypedInsertsItsPairButNotAfterAWord(t *testing.T) {
	h := closingEditor(t, "s := ", SingleSelection(5, 5))
	h.typeText("\"")
	h.wantDoc("s := \"\"", 6)
	h.typeText("\"")
	h.wantDoc("s := \"\"", 7)
	h = closingEditor(t, "don", SingleSelection(3, 3))
	h.typeText("'")
	h.wantDoc("don'", 4)
}

func TestABracketBeforeAWordIsNotClosed(t *testing.T) {
	h := closingEditor(t, "x", SingleSelection(0, 0))
	h.typeText("(")
	h.wantDoc("(x", 1)
}

func TestBackspaceInAnEmptyPairDeletesBoth(t *testing.T) {
	h := closingEditor(t, "f", SingleSelection(1, 1))
	h.typeText("[")
	h.press(key.NameDeleteBackward, 0)
	h.wantDoc("f", 1)
}
