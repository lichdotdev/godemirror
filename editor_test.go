package godemirror

import (
	"image"
	"strconv"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

func newEditorHarness(t *testing.T, c EditorStateConfig) *harness {
	t.Helper()
	e := newTestEditor(t, c)
	h := &harness{t: t, v: e.view, ed: e, size: image.Pt(400, 200), now: time.Unix(1, 0)}
	h.frame()
	return h
}

func TestADispatchShowsOnTheNextFrame(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "one\ntwo"})
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{{From: 4, To: 7, Insert: "2\nthree\n"}}})
	if h.v.LineCount() != 2 {
		t.Fatalf("the view changed before a frame: %d lines", h.v.LineCount())
	}
	h.frame()
	var lines []string
	for i := range h.v.LineCount() {
		lines = append(lines, h.v.LineText(i))
	}
	if got := len(lines); got != 4 || lines[1] != "2" || lines[2] != "three" || lines[3] != "" {
		t.Fatalf("lines %q, want one 2 three and an empty last line", lines)
	}
}

func TestOnUpdateHearsOfTheFrameTransactionsAfterIt(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "abc"})
	var updates []Update
	h.ed.OnUpdate = func(u Update) { updates = append(updates, u) }
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(0, "x")}, UserEvent: "input.paste"})
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{del(3, 4)}, Selection: SingleSelection(1, 1)})
	if len(updates) != 0 {
		t.Fatal("OnUpdate was called by Dispatch")
	}
	h.frame()
	if len(updates) != 1 {
		t.Fatalf("%d updates after a frame, want 1", len(updates))
	}
	u := updates[0]
	if u.StartState.Doc() != "abc" || u.State.Doc() != "xab" || u.Changes.String() != "0:1 2 1:0" ||
		len(u.Transactions) != 2 || !u.DocChanged() || !u.SelectionSet() || !u.Transactions[0].IsUserEvent("input") {
		t.Fatalf("update %q -> %q by %s, %d transactions", u.StartState.Doc(), u.State.Doc(), u.Changes, len(u.Transactions))
	}
	h.frame()
	if len(updates) != 1 {
		t.Fatal("a frame with nothing dispatched reported an update")
	}
}

func TestADispatchFromOnUpdateIsReportedAfterTheNextFrame(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "a"})
	var docs []string
	h.ed.OnUpdate = func(u Update) {
		docs = append(docs, u.State.Doc())
		if len(docs) == 1 {
			dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(len(u.State.Doc()), "c")}})
		}
	}
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(1, "b")}})
	h.frame()
	if !h.redrawRequested() {
		t.Error("a dispatch from OnUpdate did not ask for a frame")
	}
	h.frame()
	if len(docs) != 2 || docs[0] != "ab" || docs[1] != "abc" {
		t.Fatalf("updates saw %q, want ab then abc", docs)
	}
}

func TestSetStateIsNotAnUpdate(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "a"})
	called := false
	h.ed.OnUpdate = func(Update) { called = true }
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(0, "x")}})
	if err := h.ed.SetState(EditorStateConfig{Doc: "fresh"}); err != nil {
		t.Fatal(err)
	}
	h.frame()
	if called || h.v.LineText(0) != "fresh" {
		t.Fatalf("OnUpdate called %v, line %q", called, h.v.LineText(0))
	}
}

func TestADragSelectsInBytesAcrossTabsAndWideCharacters(t *testing.T) {
	// "é" is two bytes and one cell, "世" three bytes and two cells, a tab
	// four cells.
	h := newEditorHarness(t, EditorStateConfig{Doc: "é世x\n\tyz"})
	h.pointer(pointer.Press, h.at(0, 1), 0)
	h.pointer(pointer.Move, h.at(1, 5), 0)
	h.pointer(pointer.Release, h.at(1, 5), 0)
	main := h.ed.State().Selection().Main()
	if main.Anchor() != 2 || main.Head() != 9 {
		t.Fatalf("selection %d..%d, want 2..9", main.Anchor(), main.Head())
	}
}

func TestAClickCollapsesARangeSetAndShiftExtendsTheMainOne(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{
		Doc:                     "0123456789",
		AllowMultipleSelections: true,
		Selection:               NewSelection([]SelectionRange{Range(1, 2), Range(4, 5)}, 1),
	})
	if len(h.v.others) != 1 {
		t.Fatalf("%d other ranges painted, want 1", len(h.v.others))
	}
	h.click(h.at(0, 8), key.ModShift)
	if got := rangesOf(h.ed.State().Selection()); got != "1/2,4/8" {
		t.Fatalf("shift click left %s, want 1/2,4/8", got)
	}
	h.click(h.at(0, 3), 0)
	if got := rangesOf(h.ed.State().Selection()); got != "3/3" || len(h.v.others) != 0 {
		t.Fatalf("click left %s and %d painted others, want 3/3 alone", got, len(h.v.others))
	}
}

func TestAnEditKeepsTheColorsOfTheLinesItLeftAlone(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Filename: "a.go", Doc: "package a\n\nvar x = 1\n\nfunc f() {}\n"})
	waitForHighlight(h)
	before := h.v.doc.tags
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{{From: 19, To: 20, Insert: "2"}}})
	h.frame()
	// The edit's own highlighting may already land in this frame, with tags
	// equal to the carried ones but interned anew, so the tags are compared
	// by name.
	after := h.v.doc.tags
	if after[0][0] == nil || tagNames(after[0][0]) != tagNames(before[0][0]) || tagNames(after[4][0]) != tagNames(before[4][0]) {
		t.Fatal("an untouched line lost its colors before the highlighting landed")
	}
	waitForHighlight(h)
	if h.v.doc.tags[2][8] == nil {
		t.Fatal("the edited line was never highlighted")
	}
}

func TestFormatNumberSpellsTheGutterFromTheShownState(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "a\nb\nc\nd"})
	h.ed.FormatNumber = func(lineNo int, s EditorState) string {
		cursor := s.LineAt(s.Selection().Main().Head()).Number
		if lineNo == cursor {
			return strconv.Itoa(lineNo)
		}
		return strconv.Itoa(max(lineNo-cursor, cursor-lineNo))
	}
	h.frame()
	if got := []string{h.v.label(0), h.v.label(1), h.v.label(3)}; got[0] != "1" || got[1] != "1" || got[2] != "3" {
		t.Fatalf("labels %q, want the cursor's line absolute and the rest counted from it", got)
	}
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(4, 4)})
	h.frame()
	if got := []string{h.v.label(0), h.v.label(2), h.v.label(3)}; got[0] != "2" || got[1] != "3" || got[2] != "1" {
		t.Fatalf("after the cursor moved, labels %q", got)
	}
	if _, ok := h.v.gutter[gutterKey{3, "1"}]; !ok {
		t.Fatal("the label drawn is not the one cached for its new text")
	}
	h.ed.FormatNumber = nil
	h.frame()
	if got := h.v.label(0); got != "1" {
		t.Fatalf("without FormatNumber line 1 reads %q", got)
	}
}
