package godemirror

import (
	"testing"

	"gioui.org/io/key"
)

func tabEditor(t *testing.T, doc string, sel EditorSelection) *harness {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: doc, IndentUnit: "\t"}, sel)
	h.ed.IndentWithTab = true
	h.frame()
	return h
}

func TestTabInsertsAnIndentUnitAtTheCursor(t *testing.T) {
	h := tabEditor(t, "ab", SingleSelection(1, 1))
	h.press(key.NameTab, 0)
	if got := h.ed.State().Doc(); got != "a\tb" {
		t.Fatalf("doc %q", got)
	}
	if !h.router.Source().Focused(h.v) {
		t.Fatal("Tab moved the focus away")
	}
}

func TestTabIndentsTheSelectedLinesAndShiftTabOutdentsThem(t *testing.T) {
	h := tabEditor(t, "a\n\tb\nc", SingleSelection(0, 4))
	h.press(key.NameTab, 0)
	if got := h.ed.State().Doc(); got != "\ta\n\t\tb\nc" {
		t.Fatalf("after Tab %q", got)
	}
	h.press(key.NameTab, key.ModShift)
	h.press(key.NameTab, key.ModShift)
	if got := h.ed.State().Doc(); got != "a\nb\nc" {
		t.Fatalf("after Shift+Tab twice %q", got)
	}
}
