package godemirror

import (
	"testing"

	"gioui.org/io/key"
)

func bracketEditor(t *testing.T, doc string, at int) *harness {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: doc, IndentUnit: "\t", BracketIndent: true}, SingleSelection(at, at))
	return h
}

func TestEnterAfterAnOpenBracketIndentsOneUnitMore(t *testing.T) {
	h := bracketEditor(t, "\tif ok {", 8)
	h.press(key.NameReturn, 0)
	if got := h.ed.State().Doc(); got != "\tif ok {\n\t\t" {
		t.Fatalf("doc %q", got)
	}
}

func TestEnterBetweenBracketsIndentsTheMiddleLine(t *testing.T) {
	h := bracketEditor(t, "f() {}", 5)
	h.press(key.NameReturn, 0)
	if got := h.ed.State().Doc(); got != "f() {\n\t\n}" {
		t.Fatalf("doc %q", got)
	}
	if _, hd := h.mainRange(); hd != 7 {
		t.Fatalf("cursor at %d, want 7 on the middle line", hd)
	}
}

func TestEnterAfterAClosedBracketKeepsTheIndentation(t *testing.T) {
	h := bracketEditor(t, "\tf(x)", 5)
	h.press(key.NameReturn, 0)
	if got := h.ed.State().Doc(); got != "\tf(x)\n\t" {
		t.Fatalf("doc %q", got)
	}
}

func TestAClosingBracketTypedFirstOnALineOutdentsIt(t *testing.T) {
	h := bracketEditor(t, "\tif ok {\n\t\treturn\n\t\t", 20)
	h.typeText("}")
	if got := h.ed.State().Doc(); got != "\tif ok {\n\t\treturn\n\t}" {
		t.Fatalf("doc %q", got)
	}
	if _, hd := h.mainRange(); hd != 20 {
		t.Fatalf("cursor at %d, want after the }", hd)
	}
}
