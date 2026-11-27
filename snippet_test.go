package godemirror

import (
	"context"
	"reflect"
	"testing"

	"gioui.org/io/key"
)

func TestSnippetsParse(t *testing.T) {
	for _, tc := range []struct {
		in, indent, text string
		fields           []snippetField
	}{
		{"Printf(${1:format}, ${2:a})$0", "", "Printf(format, a)", []snippetField{{1, 7, 13}, {2, 15, 16}, {0, 17, 17}}},
		{"for ${1:i} := range ${2:n} {\n\t$0\n}", "\t", "for i := range n {\n\t\t\n\t}", []snippetField{{1, 4, 5}, {2, 15, 16}, {0, 21, 21}}},
		{"${1|one,two|} \\$5 \\} ${TM_X} ${V:def}", "", "one $5 }  def", []snippetField{{1, 0, 3}}},
		{"${1:outer ${2:in}}", "", "outer in", []snippetField{{2, 6, 8}, {1, 0, 8}}},
	} {
		text, fields := parseSnippet(tc.in, tc.indent)
		if text != tc.text || !reflect.DeepEqual(fields, tc.fields) {
			t.Errorf("%q: got %q %v, want %q %v", tc.in, text, fields, tc.text, tc.fields)
		}
	}
}

func TestTabVisitsASnippetsFields(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "x"}, SingleSelection(1, 1))
	if err := h.ed.InsertSnippet(1, 1, "(${1:a}, ${2:b})$0;"); err != nil {
		t.Fatal(err)
	}
	h.frame()
	if a, hd := h.mainRange(); a != 2 || hd != 3 {
		t.Fatalf("selection %d..%d, want the first field 2..3", a, hd)
	}
	h.typeText("alpha")
	h.press(key.NameTab, 0)
	if a, hd := h.mainRange(); a != 9 || hd != 10 {
		t.Fatalf("after Tab %d..%d, want b at 9..10", a, hd)
	}
	h.press(key.NameTab, key.ModShift)
	if a, hd := h.mainRange(); a != 2 || hd != 7 {
		t.Fatalf("after Shift+Tab %d..%d, want alpha at 2..7", a, hd)
	}
	h.press(key.NameTab, 0)
	h.press(key.NameTab, 0)
	if _, hd := h.mainRange(); hd != 11 || h.ed.snippet != nil {
		t.Fatalf("cursor %d, snippet %v; want $0 at 11 and the snippet over", hd, h.ed.snippet)
	}
	h.press(key.NameTab, 0)
	if got := h.ed.State().Doc(); got != "x(alpha, b);" {
		t.Fatalf("doc %q: Tab after the snippet must not reach it", got)
	}
}

func TestEscapeOrLeavingEndsASnippet(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "x\ny"}, SingleSelection(1, 1))
	_ = h.ed.InsertSnippet(1, 1, "(${1:a}, ${2:b})")
	h.frame()
	h.press(key.NameEscape, 0)
	if h.ed.snippet != nil {
		t.Fatal("Escape left the snippet on")
	}
	_ = h.ed.InsertSnippet(1, 1, "[${1:c}]")
	h.frame()
	// The first Down collapses the selected field, as CodeMirror's
	// cursorLineDown does; the second leaves the line.
	h.press(key.NameDownArrow, 0)
	h.press(key.NameDownArrow, 0)
	if h.ed.snippet != nil {
		t.Fatal("leaving the snippet left it on")
	}
}

func TestASnippetCompletionInsertsItsFields(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "fmt.Pr"}, SingleSelection(6, 6))
	h.ed.Complete = func(_ context.Context, req CompletionRequest) CompletionResult {
		return CompletionResult{Items: []CompletionItem{{Label: "Printf", Insert: "Printf(${1:format})$0", Snippet: true, From: 4, To: 6}}}
	}
	h.press(key.NameSpace, key.ModShortcut)
	h.waitForCompletions()
	h.press(key.NameReturn, 0)
	if got := h.ed.State().Doc(); got != "fmt.Printf(format)" {
		t.Fatalf("doc %q", got)
	}
	if a, hd := h.mainRange(); a != 11 || hd != 17 {
		t.Fatalf("selection %d..%d, want format selected", a, hd)
	}
}

// A completion inside a field, a snippet with no fields of its own as
// gopls sends a variable, ended the snippet it was typed in.
func TestASnippetWithNoFieldsKeepsTheOneItIsTypedIn(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "x"}, SingleSelection(1, 1))
	if err := h.ed.InsertSnippet(1, 1, "f(${1})"); err != nil {
		t.Fatal(err)
	}
	h.frame()
	if err := h.ed.InsertSnippet(3, 3, "started"); err != nil {
		t.Fatal(err)
	}
	h.frame()
	h.press(key.NameTab, 0)
	if got, at := h.ed.State().Doc(), h.ed.State().Selection().Main().Head(); got != "xf(started)" || at != len(got) {
		t.Fatalf("doc %q, cursor %d; want Tab to leave the outer snippet after its )", got, at)
	}
}
