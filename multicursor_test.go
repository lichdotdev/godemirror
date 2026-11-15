package godemirror

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

func TestCtrlAltArrowsAddACursorAboveAndBelow(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "abcdef\nab\nabcdef", AllowMultipleSelections: true}, SingleSelection(4, 4))
	h.press(key.NameDownArrow, key.ModShortcut|key.ModAlt)
	if got := rangesOf(h.ed.State().Selection()); got != "4/4,9/9" || h.ed.State().Selection().MainIndex() != 1 {
		t.Fatalf("a cursor below left %s main %d, want 4/4,9/9 main 1", got, h.ed.State().Selection().MainIndex())
	}
	h.press(key.NameDownArrow, key.ModShortcut|key.ModAlt)
	if got := rangesOf(h.ed.State().Selection()); got != "4/4,9/9,14/14" {
		t.Fatalf("another below left %s, want the goal column kept on the last line, 14", got)
	}
	h.press(key.NameUpArrow, key.ModShortcut|key.ModAlt)
	if got := rangesOf(h.ed.State().Selection()); got != "4/4,9/9,14/14" {
		t.Fatalf("a cursor above where there is one already left %s", got)
	}
	h.press(key.NameEscape, 0)
	if got := rangesOf(h.ed.State().Selection()); got != "14/14" {
		t.Fatalf("Escape left %s, want the main cursor alone", got)
	}
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(0, 3)})
	h.press(key.NameEscape, 0)
	if got := rangesOf(h.ed.State().Selection()); got != "3/3" {
		t.Fatalf("Escape on a range left %s, want its head", got)
	}
}

func TestAnAltDragSelectsARectangleWhenAsked(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "abcdef\nab\nabcdef", AllowMultipleSelections: true})
	h.pointer(pointer.Press, h.at(0, 1), key.ModAlt)
	h.pointer(pointer.Move, h.at(2, 4), key.ModAlt)
	h.pointer(pointer.Release, h.at(2, 4), key.ModAlt)
	if got := rangesOf(h.ed.State().Selection()); got == "1/1,8/8,12/14" || len(h.ed.State().Selection().Ranges()) != 1 {
		t.Fatalf("without RectangularSelection an Alt drag left %s, want one range", got)
	}
	h.ed.RectangularSelection = true
	h.pointer(pointer.Press, h.at(0, 1), key.ModAlt)
	h.pointer(pointer.Move, h.at(2, 4), key.ModAlt)
	h.pointer(pointer.Release, h.at(2, 4), key.ModAlt)
	if got := rangesOf(h.ed.State().Selection()); got != "1/4,8/9,11/14" {
		t.Fatalf("an Alt drag selected %s, want columns 1 to 4 of each line, 1/4,8/9,11/14", got)
	}
}

func TestAnAltDragPastTheLinesCountsColumnsFromThePointer(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "ab\nabcdefgh", AllowMultipleSelections: true})
	h.ed.RectangularSelection = true
	h.pointer(pointer.Press, h.at(0, 5), key.ModAlt)
	h.pointer(pointer.Move, h.at(1, 7), key.ModAlt)
	if got := rangesOf(h.ed.State().Selection()); got != "2/2,8/10" {
		t.Fatalf("an Alt drag from past a line's end selected %s, want 2/2,8/10", got)
	}
}

func TestTheCrosshairShowsWhileAltIsHeldWhenAsked(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "abc"})
	h.send(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: h.at(0, 1), Modifiers: key.ModAlt})
	if c := h.router.Cursor(); c != pointer.CursorText {
		t.Fatalf("without CrosshairCursor the pointer is %v", c)
	}
	h.ed.CrosshairCursor = true
	h.send(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: h.at(0, 2), Modifiers: key.ModAlt})
	if c := h.router.Cursor(); c != pointer.CursorCrosshair {
		t.Fatalf("with Alt held the pointer is %v, want a crosshair", c)
	}
	h.send(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: h.at(0, 1)})
	if c := h.router.Cursor(); c != pointer.CursorText {
		t.Fatalf("with Alt released the pointer is %v", c)
	}
}
