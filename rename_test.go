package godemirror

import (
	"context"
	"errors"
	"testing"

	"gioui.org/io/key"
)

func renamingEditor(t *testing.T, doc string, at int) (*harness, chan string) {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: doc}, SingleSelection(at, at))
	names := make(chan string, 4)
	h.ed.Rename = func(_ context.Context, s EditorState, pos int, name string) ([]DocumentEdit, error) {
		names <- name
		if name == "bad" {
			return nil, errors.New("cannot rename to bad")
		}
		return []DocumentEdit{
			{Edits: []TextEdit{
				{Start: LineColumn{0, 4}, End: LineColumn{0, 7}, Text: name},
				{Start: LineColumn{1, 0}, End: LineColumn{1, 3}, Text: name},
			}},
			{URI: "file:///b.go", Edits: []TextEdit{{Start: LineColumn{9, 1}, End: LineColumn{9, 4}, Text: name}}},
		}, nil
	}
	return h, names
}

func TestF2RenamesTheSymbolHereAndHandsTheRestOver(t *testing.T) {
	h, names := renamingEditor(t, "var old = 1\nold++", 5)
	var elsewhere []DocumentEdit
	h.ed.OnWorkspaceEdit = func(d []DocumentEdit) { elsewhere = d }
	h.press(key.NameF2, 0)
	if in := h.ed.rename.input; in == nil || in.text != "old" {
		t.Fatalf("input %+v, want it holding old", in)
	}
	h.typeText("fresh")
	if h.ed.State().Doc() != "var old = 1\nold++" {
		t.Fatal("typing in the input reached the document")
	}
	h.press(key.NameReturn, 0)
	h.until("renamed", func() bool { return h.ed.State().Doc() == "var fresh = 1\nfresh++" })
	if <-names != "fresh" || len(elsewhere) != 1 || elsewhere[0].URI != "file:///b.go" {
		t.Fatalf("OnWorkspaceEdit got %+v", elsewhere)
	}
	if h.ed.UndoDepth() != 1 {
		t.Fatalf("undo depth %d, want the rename as one event", h.ed.UndoDepth())
	}
}

func TestEscapeOrAnUnchangedNameRenamesNothing(t *testing.T) {
	h, names := renamingEditor(t, "var old = 1\nold++", 5)
	h.press(key.NameF2, 0)
	h.typeText("x")
	h.press(key.NameEscape, 0)
	h.press(key.NameF2, 0)
	h.press(key.NameReturn, 0)
	h.advance(0)
	if len(names) != 0 || h.ed.rename.input != nil {
		t.Fatal("Escape or the old name asked for a rename")
	}
}

func TestBackspaceEditsTheNameAndAnErrorShows(t *testing.T) {
	h, _ := renamingEditor(t, "var old = 1\nold++", 5)
	h.press(key.NameF2, 0)
	h.typeText("bada")
	h.press(key.NameDeleteBackward, 0)
	h.press(key.NameReturn, 0)
	h.until("failed", func() bool { return h.ed.rename.err != "" })
	if h.ed.rename.err != "cannot rename to bad" {
		t.Fatalf("error %q", h.ed.rename.err)
	}
	h.typeText("z")
	if h.ed.rename.err != "" {
		t.Fatal("the error stayed after an edit")
	}
}
