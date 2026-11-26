package godemirror

import (
	"testing"

	"gioui.org/io/key"
)

func TestDiagnosticsFollowEditsAndDieWithTheirText(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "foo bar baz"}, SingleSelection(0, 0))
	h.ed.SetDiagnostics([]Diagnostic{
		{From: 4, To: 7, Message: "bar"},
		{From: 8, To: 11, Message: "baz", Severity: SeverityWarning},
	})
	h.typeText("xx")
	got := h.ed.Diagnostics()
	if len(got) != 2 || got[0].From != 6 || got[0].To != 9 || got[1].From != 10 {
		t.Fatalf("after typing before them: %+v", got)
	}
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{{From: 6, To: 9}}})
	got = h.ed.Diagnostics()
	if len(got) != 1 || got[0].Message != "baz" || got[0].From != 7 || got[0].To != 10 {
		t.Fatalf("after deleting bar: %+v, want baz alone at 7..10", got)
	}
}

func TestSetDiagnosticsClampsToTheDocument(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "aé"})
	h.ed.SetDiagnostics([]Diagnostic{{From: 2, To: 99}, {From: -3, To: 1}})
	got := h.ed.Diagnostics()
	if got[0].From != 0 || got[0].To != 1 || got[1].From != 1 || got[1].To != 3 {
		t.Fatalf("clamped to %+v, want 0..1 and 1..3 (2 is inside é)", got)
	}
}

func TestSetStateDropsTheDiagnostics(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "abc"})
	h.ed.SetDiagnostics([]Diagnostic{{From: 0, To: 1}})
	if err := h.ed.SetState(EditorStateConfig{Doc: "other"}); err != nil {
		t.Fatal(err)
	}
	if len(h.ed.Diagnostics()) != 0 {
		t.Fatal("a new state kept the old document's diagnostics")
	}
}

func TestF8GoesToTheNextDiagnosticAndWrapsAround(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "one two three four"}, SingleSelection(5, 5))
	h.ed.SetDiagnostics([]Diagnostic{{From: 0, To: 3}, {From: 8, To: 13}, {From: 14, To: 18}})
	for _, want := range []int{8, 14, 0} {
		h.press(key.NameF8, 0)
		if _, hd := h.mainRange(); hd != want {
			t.Fatalf("F8 went to %d, want %d", hd, want)
		}
	}
	h.press(key.NameF8, key.ModShift)
	if _, hd := h.mainRange(); hd != 14 {
		t.Fatalf("Shift+F8 from 0 went to %d, want the last, 14", hd)
	}
	if !h.ed.jumped {
		t.Fatal("a jump does not show the diagnostic it went to")
	}
	h.press(key.NameLeftArrow, 0)
	if h.ed.jumped {
		t.Fatal("the diagnostic still shows after the cursor moved on")
	}
}

func TestNextDiagnosticTakesOnlyItsSeverity(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "one two three four"})
	h.ed.SetDiagnostics([]Diagnostic{
		{From: 4, To: 7, Severity: SeverityWarning},
		{From: 8, To: 13},
		{From: 14, To: 18, Severity: SeverityWarning},
	})
	if !h.ed.NextDiagnostic(SeverityError) {
		t.Fatal("no error found")
	}
	if hd := h.ed.State().Selection().Main().Head(); hd != 8 {
		t.Fatalf("the next error is at %d, want 8, an unset severity being an error", hd)
	}
	h.ed.NextDiagnostic(SeverityWarning)
	if hd := h.ed.State().Selection().Main().Head(); hd != 14 {
		t.Fatalf("the next warning is at %d, want 14", hd)
	}
	if h.ed.PrevDiagnostic(SeverityHint) {
		t.Fatal("found a hint where there is none")
	}
}
