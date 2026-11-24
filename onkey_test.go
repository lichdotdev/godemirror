package godemirror

import (
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
)

// onKeyEditor is a focused editor whose OnKey gathers what it sees in seen
// and returns take.
func onKeyEditor(t *testing.T, doc string, take bool) (*harness, *[]event.Event) {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: doc}, SingleSelection(1, 1))
	seen := &[]event.Event{}
	h.ed.OnKey = func(_ layout.Context, ev event.Event) bool {
		*seen = append(*seen, ev)
		return take
	}
	h.frame()
	return h, seen
}

func TestOnKeyTakesKeysAndTextBeforeTheEditor(t *testing.T) {
	h, seen := onKeyEditor(t, "abc", true)
	h.press(key.NameRightArrow, 0)
	h.send(key.Event{Name: key.NameRightArrow, State: key.Release})
	h.typeText("x")
	h.press(key.NameDeleteBackward, 0)
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(0, 2)})
	h.frame()
	h.press(keyCut, key.ModShortcut)
	if got := h.ed.State().Doc(); got != "abc" {
		t.Fatalf("doc %q after taken keys, want abc untouched", got)
	}
	if a, hd := h.mainRange(); a != 0 || hd != 2 {
		t.Fatalf("selection %d..%d, want the 0..2 dispatched", a, hd)
	}
	want := []event.Event{
		key.Event{Name: key.NameRightArrow, State: key.Press},
		key.Event{Name: key.NameRightArrow, State: key.Release},
		key.EditEvent{Range: key.Range{Start: 1, End: 1}, Text: "x"},
		key.Event{Name: key.NameDeleteBackward, State: key.Press},
		key.Event{Name: keyCut, Modifiers: key.ModShortcut, State: key.Press},
	}
	if len(*seen) != len(want) {
		t.Fatalf("OnKey saw %v, want %v", *seen, want)
	}
	for i := range want {
		if (*seen)[i] != want[i] {
			t.Fatalf("OnKey saw %v, want %v", *seen, want)
		}
	}
}

func TestATakenEditLeavesTheSelectionWhereItWas(t *testing.T) {
	h, _ := onKeyEditor(t, "abc", true)
	// typeText moves the text input's selection past the edit, as Gio's
	// window does whether the edit was applied or not.
	h.typeText("xy")
	if a, hd := h.mainRange(); a != 1 || hd != 1 {
		t.Fatalf("selection %d..%d after a taken edit, want 1..1", a, hd)
	}
	h.send(key.SelectionEvent{Start: 3, End: 3})
	if _, hd := h.mainRange(); hd != 3 {
		t.Fatalf("an input method's selection after it moved the cursor to %d, want 3", hd)
	}
}

func TestOnKeyReturningFalseLeavesTheEditorToAct(t *testing.T) {
	h, seen := onKeyEditor(t, "abc", false)
	h.press(key.NameRightArrow, 0)
	h.send(key.Event{Name: "Q", Modifiers: key.ModCtrl | key.ModAlt, State: key.Press})
	h.typeText("x")
	if got := h.ed.State().Doc(); got != "abxc" {
		t.Fatalf("doc %q, want abxc", got)
	}
	if _, hd := h.mainRange(); hd != 3 {
		t.Fatalf("cursor at %d, want 3", hd)
	}
	if len(*seen) != 3 || (*seen)[1] != (key.Event{Name: "Q", Modifiers: key.ModCtrl | key.ModAlt, State: key.Press}) {
		t.Fatalf("OnKey saw %v, want the arrow, Ctrl+Alt+Q no binding names, and the edit", *seen)
	}
}

func TestOnKeyTakesTabFromFocusMoving(t *testing.T) {
	h, seen := onKeyEditor(t, "abc", true)
	for _, mods := range []key.Modifiers{0, key.ModShift} {
		// Gio's window marks Tab and Shift+Tab as SystemEvents, which move
		// focus unless a filter names them.
		h.router.Queue(input.SystemEvent{Event: key.Event{Name: key.NameTab, Modifiers: mods, State: key.Press}})
		h.frame()
	}
	if len(*seen) != 2 {
		t.Fatalf("OnKey saw %v, want Tab and Shift+Tab", *seen)
	}
}
