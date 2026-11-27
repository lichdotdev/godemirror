package godemirror

import (
	"context"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
)

func actionEditor(t *testing.T) (*harness, chan []Diagnostic) {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "ok\nx := y\n"}, SingleSelection(0, 0))
	asked := make(chan []Diagnostic, 8)
	h.ed.CodeActions = func(_ context.Context, _ EditorState, from, to int, diags []Diagnostic) []CodeAction {
		asked <- diags
		if len(diags) == 0 {
			return nil
		}
		return []CodeAction{{Title: "Declare y", Data: "y"}, {Title: "Use _", Data: "_"}}
	}
	h.ed.ApplyCodeAction = func(_ context.Context, _ EditorState, a CodeAction) ([]DocumentEdit, error) {
		return []DocumentEdit{{Edits: []TextEdit{{Start: LineColumn{1, 5}, End: LineColumn{1, 6}, Text: a.Data.(string)}}}}, nil
	}
	h.ed.SetDiagnostics([]Diagnostic{{From: 8, To: 9, Message: "undefined: y"}})
	return h, asked
}

func TestTheCursorOnADiagnosticShowsABulbAndCtrlDotListsTheActions(t *testing.T) {
	h, asked := actionEditor(t)
	h.frame()
	if len(asked) != 0 {
		t.Fatal("actions asked for a line with no diagnostic")
	}
	h.press(key.NameDownArrow, 0)
	h.until("offered", func() bool { return len(h.ed.actions.items) == 2 })
	if d := <-asked; len(d) != 1 || d[0].Message != "undefined: y" {
		t.Fatalf("asked with %+v", d)
	}
	if h.ed.popups.boxes[bulbPopup].Empty() {
		t.Fatal("no bulb in the gutter")
	}
	h.press(".", key.ModShortcut)
	h.until("listed", func() bool { return h.ed.actions.open && len(h.ed.actions.items) == 2 })
	h.press(key.NameDownArrow, 0)
	h.press(key.NameReturn, 0)
	h.until("applied", func() bool { return h.ed.State().Doc() == "ok\nx := _\n" })
}

func TestClickingTheBulbAndAnActionAppliesIt(t *testing.T) {
	h, _ := actionEditor(t)
	h.press(key.NameDownArrow, 0)
	h.until("offered", func() bool { return !h.ed.popups.boxes[bulbPopup].Empty() })
	h.click(centre(h.ed.popups.boxes[bulbPopup]), 0)
	h.frame()
	box := h.ed.popups.boxes[actionsPopup]
	if box.Empty() {
		t.Fatal("a click on the bulb listed nothing")
	}
	h.click(f32.Pt(float32(box.Min.X+5), float32(box.Min.Y+2)), 0)
	h.until("applied", func() bool { return h.ed.State().Doc() == "ok\nx := y\n" && h.ed.UndoDepth() == 1 })
}

// The bulb was drawn from the gutter's first cell, nine pixels wide,
// over the line number's first digit.
func TestTheBulbNeverCoversTheLineNumber(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: strings.Repeat("ok\n", 22) + "x := y\n"}, SingleSelection(0, 0))
	h.ed.CodeActions = func(context.Context, EditorState, int, int, []Diagnostic) []CodeAction {
		return []CodeAction{{Title: "Declare y"}}
	}
	h.ed.SetDiagnostics([]Diagnostic{{From: 71, To: 72, Message: "undefined: y"}})
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(66, 66), ScrollIntoView: true})
	h.until("offered", func() bool { return !h.ed.popups.boxes[bulbPopup].Empty() })
	g := h.ed.Geometry()
	bulb := h.ed.popups.boxes[bulbPopup]
	numberEnd := g.GutterWidth - g.CellWidth
	if bulb.Min.X < numberEnd || bulb.Max.X > g.GutterWidth {
		t.Fatalf("bulb at x %d..%d, want it in the marks cell %d..%d, clear of the number ending at %d",
			bulb.Min.X, bulb.Max.X, numberEnd, g.GutterWidth, numberEnd)
	}
}
