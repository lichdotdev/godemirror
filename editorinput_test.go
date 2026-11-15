package godemirror

import (
	"io"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/unit"
)

// focusedEditor is an editor harness holding focus, with sel set, its
// updates' user events gathered in events.
func focusedEditor(t *testing.T, c EditorStateConfig, sel EditorSelection) (*harness, *[]string) {
	t.Helper()
	h := newEditorHarness(t, c)
	h.router.Source().Execute(key.FocusCmd{Tag: h.v})
	h.frame()
	if len(sel.ranges) > 0 {
		dispatch(t, h.ed, TransactionSpec{Selection: sel})
		h.frame()
	}
	events := &[]string{}
	h.ed.OnUpdate = func(u Update) {
		for _, tr := range u.Transactions {
			*events = append(*events, tr.UserEvent)
		}
	}
	return h, events
}

func (h *harness) press(name key.Name, mods key.Modifiers) {
	h.t.Helper()
	h.send(key.Event{Name: name, Modifiers: mods, State: key.Press})
}

// typeText types s as Gio's window does: an edit of the text input's
// selection, and that selection moved after it.
func (h *harness) typeText(s string) {
	h.t.Helper()
	sel := h.router.EditorState().Selection.Range
	h.send(key.EditEvent{Range: sel, Text: s})
	at := min(sel.Start, sel.End) + len([]rune(s))
	h.send(key.SelectionEvent{Start: at, End: at})
}

func (h *harness) mainRange() (anchor, head int) {
	m := h.ed.State().Selection().Main()
	return m.Anchor(), m.Head()
}

func TestTypingInsertsAtEveryCursor(t *testing.T) {
	h, events := focusedEditor(t, EditorStateConfig{Doc: "ab\ncd", AllowMultipleSelections: true},
		NewSelection([]SelectionRange{Cursor(1), Cursor(4)}, 0))
	h.typeText("X")
	if got := h.ed.State().Doc(); got != "aXb\ncXd" {
		t.Fatalf("doc %q, want aXb\\ncXd", got)
	}
	if got := rangesOf(h.ed.State().Selection()); got != "2/2,6/6" {
		t.Errorf("ranges %s, want 2/2,6/6", got)
	}
	if len(*events) == 0 || (*events)[0] != "input.type" {
		t.Errorf("events %q, want input.type first", *events)
	}
}

func TestACompositionIsTypedAsOne(t *testing.T) {
	h, events := focusedEditor(t, EditorStateConfig{Doc: "a"}, SingleSelection(1, 1))
	h.send(key.CompositionEvent{Start: 1, End: 1})
	h.send(key.EditEvent{Range: key.Range{Start: 1, End: 1}, Text: "か"})
	if got := h.ed.State().Doc(); got != "aか" || (*events)[0] != "input.type.compose" {
		t.Fatalf("doc %q events %q, want aか by input.type.compose", got, *events)
	}
}

func TestTheInputMethodHearsOfTheMainRange(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "héllo"}, SingleSelection(1, 3))
	if got := h.router.EditorState().Selection.Range; got != (key.Range{Start: 1, End: 2}) {
		t.Fatalf("text input selection %+v, want runes 1 to 2", got)
	}
}

func TestArrowsMoveByClusterAndGroupAndShiftSelects(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "foo bar.baz"}, SingleSelection(0, 0))
	for _, step := range []struct {
		name   key.Name
		mods   key.Modifiers
		anchor int
		head   int
	}{
		{key.NameRightArrow, 0, 1, 1},
		{key.NameRightArrow, key.ModShortcut, 3, 3},
		{key.NameRightArrow, key.ModShortcut, 7, 7},
		{key.NameRightArrow, key.ModShortcut | key.ModShift, 7, 8},
		{key.NameLeftArrow, key.ModShift, 7, 7},
		{key.NameLeftArrow, key.ModShortcut | key.ModShift, 7, 4},
		{key.NameRightArrow, 0, 7, 7},
	} {
		h.press(step.name, step.mods)
		if a, hd := h.mainRange(); a != step.anchor || hd != step.head {
			t.Fatalf("%s %v: selection %d..%d, want %d..%d", step.name, step.mods, a, hd, step.anchor, step.head)
		}
	}
}

func TestUpAndDownKeepTheGoalColumn(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "long line here\nab\nlong line here"}, SingleSelection(10, 10))
	h.press(key.NameDownArrow, 0)
	if _, head := h.mainRange(); head != 17 {
		t.Fatalf("down to %d, want the end of the short line, 17", head)
	}
	h.press(key.NameDownArrow, 0)
	if _, head := h.mainRange(); head != 28 {
		t.Fatalf("down again to %d, want column 10 of the last line, 28", head)
	}
	h.press(key.NameDownArrow, 0)
	if _, head := h.mainRange(); head != 32 {
		t.Fatalf("down on the last line to %d, want its end, 32", head)
	}
}

func TestHomeTogglesBetweenTheIndentationAndTheLineStart(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "    foo"}, SingleSelection(7, 7))
	for _, want := range []int{4, 0, 4} {
		h.press(key.NameHome, 0)
		if _, head := h.mainRange(); head != want {
			t.Fatalf("Home to %d, want %d", head, want)
		}
	}
	h.press(key.NameEnd, key.ModShift)
	if a, hd := h.mainRange(); a != 4 || hd != 7 {
		t.Fatalf("Shift+End selected %d..%d, want 4..7", a, hd)
	}
}

func TestEndStopsAtTheEndOfAWrappedRowFirst(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: strings.Repeat("word ", 40)})
	h.ed.Wrap = true
	h.frame()
	h.router.Source().Execute(key.FocusCmd{Tag: h.v})
	h.frame()
	rowEnd := h.v.wraps[0][1]
	h.press(key.NameEnd, 0)
	if _, head := h.mainRange(); head != rowEnd {
		t.Fatalf("End to %d, want the end of the first row, %d", head, rowEnd)
	}
	if h.ed.State().Selection().Main().Assoc() >= 0 {
		t.Fatal("the cursor at a row's end does not stick to the row before")
	}
	h.press(key.NameEnd, 0)
	if _, head := h.mainRange(); head != 200 {
		t.Fatalf("End again to %d, want the line's end, 200", head)
	}
}

func TestPageDownMovesAPageAndKeepsTheCursorWhereItWasOnScreen(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: manyLines(200)}, SingleSelection(10, 10))
	_, before := h.ed.coordsAt(10, 0)
	onScreen := before - h.v.scroll.Y
	h.press(key.NamePageDown, 0)
	_, head := h.mainRange()
	_, after := h.ed.coordsAt(head, h.ed.State().Selection().Main().Assoc())
	if rows := (after - before) / h.v.r.CellH(); rows < h.size.Y/h.v.r.CellH()-1 {
		t.Fatalf("PageDown moved %d rows in a view of %d px", rows, h.size.Y)
	}
	if got := after - h.v.scroll.Y; got != onScreen {
		t.Fatalf("the cursor's row went from %d to %d px on screen", onScreen, got)
	}
}

func TestBackspaceAndDeleteByCharacterIndentAndGroup(t *testing.T) {
	h, events := focusedEditor(t, EditorStateConfig{Doc: "    x foo.bar"}, SingleSelection(4, 4))
	h.press(key.NameDeleteBackward, 0)
	if got := h.ed.State().Doc(); got != "  x foo.bar" {
		t.Fatalf("Backspace in the indentation left %q, want it back one indent unit", got)
	}
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(13-2, 13-2)})
	h.press(key.NameDeleteBackward, key.ModShortcut)
	if got := h.ed.State().Doc(); got != "  x foo." {
		t.Fatalf("Ctrl+Backspace left %q, want the word gone", got)
	}
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(0, 0)})
	h.press(key.NameDeleteForward, 0)
	if got := h.ed.State().Doc(); got != " x foo." {
		t.Fatalf("Delete left %q", got)
	}
	if (*events)[0] != "delete.backward" {
		t.Errorf("events %q, want delete.backward first", *events)
	}
}

func TestEnterKeepsTheIndentationInTheIndentUnit(t *testing.T) {
	for _, c := range []struct{ unit, doc, want string }{
		{"\t", "\tfoo", "\tfoo\n\t"},
		{"", "\tfoo", "\tfoo\n    "},
		{"", "  {}", "  {\n  \n  }"},
	} {
		h, _ := focusedEditor(t, EditorStateConfig{Doc: c.doc, IndentUnit: c.unit}, SingleSelection(len(c.doc), len(c.doc)))
		if strings.HasSuffix(c.doc, "}") {
			dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(len(c.doc)-1, len(c.doc)-1)})
		}
		h.press(key.NameReturn, 0)
		if got := h.ed.State().Doc(); got != c.want {
			t.Errorf("Enter in %q (unit %q) gave %q, want %q", c.doc, c.unit, got, c.want)
		}
	}
}

func TestAnInvalidIndentUnitIsRefused(t *testing.T) {
	e, err := NewEditor("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range []string{"x", " \t", "\t "} {
		if err := e.SetState(EditorStateConfig{IndentUnit: unit}); err == nil {
			t.Errorf("IndentUnit %q was taken", unit)
		}
	}
}

// paste answers the editor's clipboard read with text.
func (h *harness) paste(text string) {
	h.t.Helper()
	h.press("V", key.ModShortcut)
	if !h.router.ClipboardRequested() {
		h.t.Fatal("Ctrl+V did not read the clipboard")
	}
	h.send(transfer.DataEvent{Type: "application/text", Open: func() io.ReadCloser {
		return io.NopCloser(strings.NewReader(text))
	}})
}

func TestCopyCutAndPaste(t *testing.T) {
	h, events := focusedEditor(t, EditorStateConfig{Doc: "foo bar", AllowMultipleSelections: true},
		NewSelection([]SelectionRange{Range(0, 3), Range(4, 7)}, 0))
	h.press("C", key.ModShortcut)
	if _, content, _ := h.router.WriteClipboard(); string(content) != "foo\nbar" {
		t.Fatalf("copied %q, want both ranges a line each", content)
	}
	h.press("X", key.ModShortcut)
	if got := h.ed.State().Doc(); got != " " {
		t.Fatalf("cut left %q", got)
	}
	h.paste("foo\nbar")
	if got := h.ed.State().Doc(); got != "foo bar" {
		t.Fatalf("pasting a line per cursor gave %q", got)
	}
	want := []string{"delete.cut", "input.paste"}
	if strings.Join(*events, " ") != strings.Join(want, " ") {
		t.Errorf("events %q, want %q", *events, want)
	}
}

func TestCopyingNothingCopiesTheLineAndPastesItAsALine(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "one\ntwo"}, SingleSelection(5, 5))
	h.press("C", key.ModShortcut)
	if _, content, _ := h.router.WriteClipboard(); string(content) != "two" {
		t.Fatalf("copied %q, want the cursor's line", content)
	}
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(1, 1)})
	h.paste("two")
	if got := h.ed.State().Doc(); got != "two\none\ntwo" {
		t.Fatalf("pasting the copied line gave %q, want it above the cursor's line", got)
	}
}

func TestDoubleClickSelectsAGroupTripleClickTheLine(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "foo.bar baz\nnext"})
	h.click(h.at(0, 5), 0)
	h.click(h.at(0, 5), 0)
	if a, hd := h.mainRange(); a != 4 || hd != 7 {
		t.Fatalf("double click selected %d..%d, want bar, 4..7", a, hd)
	}
	h.click(h.at(0, 5), 0)
	if a, hd := h.mainRange(); a != 0 || hd != 12 {
		t.Fatalf("triple click selected %d..%d, want the line and its break, 0..12", a, hd)
	}
}

func TestCtrlClickAddsARangeAndRemovesOne(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "0123456789", AllowMultipleSelections: true})
	h.click(h.at(0, 1), 0)
	h.click(h.at(0, 5), key.ModShortcut)
	if got := rangesOf(h.ed.State().Selection()); got != "1/1,5/5" || h.ed.State().Selection().MainIndex() != 1 {
		t.Fatalf("Ctrl+click left %s main %d, want 1/1,5/5 main 1", got, h.ed.State().Selection().MainIndex())
	}
	h.now = h.now.Add(time.Second)
	h.click(h.at(0, 5), key.ModShortcut)
	if got := rangesOf(h.ed.State().Selection()); got != "1/1" {
		t.Fatalf("Ctrl+click on a cursor left %s, want it removed", got)
	}
}

func TestTheCaretBlinksOnlyWhileFocused(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "abc"})
	waitForHighlight(h)
	h.router.WakeupTime()
	h.frame()
	if at, ok := h.router.WakeupTime(); ok {
		t.Fatalf("an unfocused editor asked for a frame at %v", at)
	}
	h.click(h.at(0, 2), 0)
	h.router.WakeupTime()
	h.frame()
	at, ok := h.router.WakeupTime()
	if !ok || at.Sub(h.now) <= 0 || at.Sub(h.now) > cursorBlinkRate/2 {
		t.Fatalf("a focused editor asked for a frame at %v (now %v), want within half a blink", at, h.now)
	}
	carets := h.ed.carets(layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}})
	x := h.ed.f.g.textX + 2*h.v.r.CellW()
	if len(carets) != 1 || carets[0].Min.X > x || carets[0].Max.X < x || carets[0].Dy() != h.v.r.CellH() {
		t.Fatalf("carets %v, want one across x=%d a row high", carets, x)
	}
}

func TestTypingAtTheEndScrollsTheCursorIntoView(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: manyLines(200)}, SingleSelection(0, 0))
	h.press(key.NameEnd, key.ModShortcut)
	h.typeText("x")
	_, top := h.ed.coordsAt(len(h.ed.State().Doc()), -1)
	if top < h.v.scroll.Y || top+h.v.r.CellH() > h.v.scroll.Y+h.size.Y {
		t.Fatalf("the cursor's row at %d is off screen, scrolled to %d", top, h.v.scroll.Y)
	}
}
