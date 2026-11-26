package godemirror

import (
	"context"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

// move moves the pointer to p with mods held, no button down.
func (h *harness) move(p f32.Point, mods key.Modifiers) {
	h.t.Helper()
	h.send(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p, Modifiers: mods})
}

// until lays the editor out, a frame a millisecond, until cond holds: what
// a hook answers on its own goroutine shows within a few frames.
func (h *harness) until(what string, cond func() bool) {
	h.t.Helper()
	for range 2000 {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
		h.advance(time.Millisecond)
	}
	h.t.Fatalf("never %s", what)
}

func TestHoverAsksAboutTheNameThePointerRestsOn(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "foo barbaz qux"})
	asked := make(chan int, 4)
	h.ed.Hover = func(_ context.Context, s EditorState, pos int) HoverInfo {
		asked <- pos
		return HoverInfo{Code: "func barbaz()", Text: s.Doc()[pos : pos+6]}
	}
	h.move(h.at(0, 6), 0)
	h.advance(hoverDelay / 2)
	if len(asked) != 0 {
		t.Fatal("Hover was asked before the pointer rested")
	}
	h.advance(hoverDelay / 2)
	h.until("answered", func() bool { return h.ed.hover.info.Text != "" })
	if pos := <-asked; pos != 4 {
		t.Fatalf("Hover asked at %d, want 4, the start of barbaz", pos)
	}
	if h.ed.hover.info.Text != "barbaz" {
		t.Fatalf("hover shows %+v", h.ed.hover.info)
	}
	h.move(h.at(0, 8), 0)
	if h.ed.hover.info.Text == "" {
		t.Fatal("the hover went away while the pointer stayed on its name")
	}
	h.move(h.at(0, 12), 0)
	if h.ed.hover.fired {
		t.Fatal("the hover stayed after the pointer left its name")
	}
}

func TestHoverOverSpaceAsksNothing(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "foo   bar"})
	h.ed.Hover = func(context.Context, EditorState, int) HoverInfo {
		t.Error("Hover asked over spaces")
		return HoverInfo{}
	}
	h.move(h.at(0, 4), 0)
	h.advance(hoverDelay)
	h.move(h.at(0, 20), 0)
	h.advance(hoverDelay)
}

func TestRestingOnADiagnosticShowsIt(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "x := unblock(v)"})
	h.ed.SetDiagnostics([]Diagnostic{{From: 5, To: 12, Message: "undefined: unblock", Source: "compiler", Code: "UndeclaredName"}})
	h.move(h.at(0, 7), 0)
	h.advance(hoverDelay)
	if d := h.ed.hover.diags; len(d) != 1 || d[0].Message != "undefined: unblock" {
		t.Fatalf("resting on the underline shows %+v", d)
	}
	h.move(h.at(0, 1), 0)
	h.advance(hoverDelay)
	if len(h.ed.hover.diags) != 0 {
		t.Fatal("a diagnostic shows away from its range")
	}
}

func definitionEditor(t *testing.T, doc string, loc Location) (*harness, chan int) {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: doc, AllowMultipleSelections: true}, SingleSelection(0, 0))
	asked := make(chan int, 4)
	h.ed.Definition = func(_ context.Context, _ EditorState, pos int) (Location, bool) {
		asked <- pos
		return loc, true
	}
	return h, asked
}

func TestCtrlClickGoesToADefinitionInTheDocument(t *testing.T) {
	h, asked := definitionEditor(t, "func f() {}\nf()", Location{Start: LineColumn{0, 5}})
	h.click(h.at(1, 0), key.ModShortcut)
	h.until("moved", func() bool { _, hd := h.mainRange(); return hd == 5 })
	if pos := <-asked; pos != 12 {
		t.Fatalf("Definition asked at %d, want 12", pos)
	}
	if n := len(h.ed.State().Selection().Ranges()); n != 1 {
		t.Fatalf("Ctrl+click on a name added a cursor: %d ranges", n)
	}
}

func TestF12SendsADefinitionElsewhereToOnDefinition(t *testing.T) {
	want := Location{URI: "file:///other.go", Start: LineColumn{3, 1}, End: LineColumn{3, 4}}
	h, asked := definitionEditor(t, "f()", want)
	var got []Location
	h.ed.OnDefinition = func(l Location) { got = append(got, l) }
	h.press(key.NameF12, 0)
	h.until("reported", func() bool { return len(got) > 0 })
	if <-asked != 0 || len(got) != 1 || got[0] != want {
		t.Fatalf("OnDefinition got %+v, want %+v", got, want)
	}
	if _, hd := h.mainRange(); hd != 0 {
		t.Fatalf("a definition elsewhere moved the cursor to %d", hd)
	}
}

func TestCtrlShowsTheNameUnderThePointerAsALink(t *testing.T) {
	h, _ := definitionEditor(t, "foo bar", Location{})
	h.move(h.at(0, 5), key.ModShortcut)
	if h.ed.link.from != 4 || h.ed.link.to != 7 || h.v.cursor != pointer.CursorPointer {
		t.Fatalf("link %v, cursor %v; want bar as a link under a pointing hand", h.ed.link, h.v.cursor)
	}
	h.send(key.Event{Name: key.NameCtrl, State: key.Release})
	if h.ed.link.from != h.ed.link.to {
		t.Fatal("the link stayed after Ctrl was let go")
	}
}

func TestCtrlClickWithoutDefinitionStillAddsACursor(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "foo bar", AllowMultipleSelections: true}, SingleSelection(0, 0))
	h.click(h.at(0, 5), key.ModShortcut)
	if n := len(h.ed.State().Selection().Ranges()); n != 2 {
		t.Fatalf("%d ranges, want 2", n)
	}
}

func TestTheGoToKeysRunTheirHooks(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "type T int\nvar x T"}, SingleSelection(18, 18))
	h.ed.TypeDefinition = func(context.Context, EditorState, int) (Location, bool) {
		return Location{Start: LineColumn{0, 5}}, true
	}
	h.ed.Implementation = func(context.Context, EditorState, int) (Location, bool) {
		return Location{URI: "file:///impl.go", Start: LineColumn{2, 0}}, true
	}
	var got []Location
	h.ed.OnDefinition = func(l Location) { got = append(got, l) }
	h.press(key.NameF12, key.ModShortcut|key.ModShift)
	h.until("moved", func() bool { _, hd := h.mainRange(); return hd == 5 })
	h.press(key.NameF12, key.ModShortcut)
	h.until("reported", func() bool { return len(got) == 1 })
	if got[0].URI != "file:///impl.go" {
		t.Fatalf("OnDefinition got %+v", got)
	}
	h.ed.ImplementationKey = Shortcut{Name: "I", Modifiers: key.ModAlt}
	h.frame() // Gio routes a key by the filters of the frame before it.
	h.press("I", key.ModAlt)
	h.until("reported again", func() bool { return len(got) == 2 })
}

func TestReferencesAreHighlightedAndHandedOver(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "a := 1\nb := a + a"}, SingleSelection(0, 0))
	h.ed.References = func(context.Context, EditorState, int) []Location {
		return []Location{
			{Start: LineColumn{0, 0}, End: LineColumn{0, 1}},
			{Start: LineColumn{1, 5}, End: LineColumn{1, 6}},
			{URI: "file:///other.go", Start: LineColumn{3, 1}, End: LineColumn{3, 2}},
		}
	}
	var got []Location
	h.ed.OnReferences = func(l []Location) { got = l }
	h.press(key.NameF12, key.ModShift)
	h.until("reported", func() bool { return len(got) == 3 })
	if spans := h.ed.occurrenceSpans(1); len(spans) != 1 || spans[0].From != 5 || spans[0].To != 6 {
		t.Fatalf("line 2 highlights %+v, want a at 5..6", spans)
	}
	h.press(key.NameEscape, 0)
	if len(h.ed.occurrenceSpans(0)) != 0 {
		t.Fatal("Escape left the references highlighted")
	}
}
