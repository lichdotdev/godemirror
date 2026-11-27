package godemirror

import (
	"testing"

	"gioui.org/io/key"
)

func TestInABrowserTheTextInputGetsNoTextAroundTheCaret(t *testing.T) {
	defer func(was bool) { snippetAroundCaret = was }(snippetAroundCaret)
	snippetAroundCaret = false
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "type T struct {\n\tName string \n}"}, SingleSelection(29, 29))
	h.send(key.SnippetEvent{Start: 0, End: 31})
	if s := h.router.EditorState().Snippet; s.Text != "" || s.Start != 29 || s.End != 29 {
		t.Fatalf("snippet %+v, want an empty one at the caret, 29", s)
	}
}

func TestTheTextInputGetsTheTextAroundTheCaretItAsks(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "abc def"}, SingleSelection(3, 3))
	h.send(key.SnippetEvent{Start: 0, End: 7})
	if s := h.router.EditorState().Snippet; s.Text != "abc def" {
		t.Fatalf("snippet %+v, want the whole range asked", s)
	}
}
