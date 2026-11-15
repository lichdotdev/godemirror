package godemirror

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
)

// The tests below are @codemirror/commands 6.11.1's test/test-history.ts,
// ported, but for those of effects and JSON, which the port has not.

type histTest struct {
	t *testing.T
	s EditorState
}

func mkHist(t *testing.T, doc string, conf *historyConfig) *histTest {
	t.Helper()
	c := EditorStateConfig{Doc: doc, AllowMultipleSelections: true}
	if conf != nil {
		c.history = *conf
	}
	s, err := newEditorState(c)
	if err != nil {
		t.Fatal(err)
	}
	return &histTest{t, s}
}

func (h *histTest) update(spec TransactionSpec) *histTest {
	h.t.Helper()
	tr, err := h.s.update(spec)
	if err != nil {
		h.t.Fatal(err)
	}
	h.s = tr.State
	return h
}

func (h *histTest) typeAt(text string, at int) *histTest {
	h.t.Helper()
	return h.update(TransactionSpec{Changes: []ChangeSpec{ins(at, text)}})
}

func (h *histTest) typ(text string) *histTest { h.t.Helper(); return h.typeAt(text, len(h.s.doc)) }

func (h *histTest) timedType(text string, at time.Time) *histTest {
	h.t.Helper()
	return h.update(TransactionSpec{Changes: []ChangeSpec{ins(len(h.s.doc), text)}, Time: at})
}

func (h *histTest) receive(text string, from, to int) *histTest {
	h.t.Helper()
	return h.update(TransactionSpec{Changes: []ChangeSpec{{From: from, To: to, Insert: text}}, SkipHistory: true})
}

func (h *histTest) isolate(i IsolateHistory) *histTest {
	h.t.Helper()
	return h.update(TransactionSpec{IsolateHistory: i})
}

func (h *histTest) sel(anchor, head int) *histTest {
	h.t.Helper()
	return h.update(TransactionSpec{Selection: SingleSelection(anchor, head)})
}

// cmd runs undo, redo, undoSelection or redoSelection and checks whether
// it applied.
func (h *histTest) cmd(name string, success bool) *histTest {
	h.t.Helper()
	var spec TransactionSpec
	var ok bool
	switch name {
	case "undo":
		spec, ok = h.s.hist.popChanges(doneBranch, h.s)
	case "redo":
		spec, ok = h.s.hist.popChanges(undoneBranch, h.s)
	case "undoSelection":
		spec, ok = h.s.hist.popSelectionOf(doneBranch, h.s)
	case "redoSelection":
		spec, ok = h.s.hist.popSelectionOf(undoneBranch, h.s)
	}
	if ok != success {
		h.t.Fatalf("%s applied %v, want %v", name, ok, success)
	}
	if ok {
		h.update(spec)
	}
	return h
}

func (h *histTest) doc(want string) *histTest {
	h.t.Helper()
	if h.s.doc != want {
		h.t.Fatalf("doc %q, want %q", h.s.doc, want)
	}
	return h
}

func (h *histTest) head(want int) *histTest {
	h.t.Helper()
	if got := h.s.sel.Main().Head(); got != want {
		h.t.Fatalf("head %d, want %d", got, want)
	}
	return h
}

func (h *histTest) depths(undo, redo int) *histTest {
	h.t.Helper()
	if u, r := h.s.hist.depth(doneBranch), h.s.hist.depth(undoneBranch); u != undo || r != redo {
		h.t.Fatalf("depths %d/%d, want %d/%d", u, r, undo, redo)
	}
	return h
}

func TestUndoAndRedoChanges(t *testing.T) {
	mkHist(t, "", nil).typ("newtext").cmd("undo", true).doc("")
	mkHist(t, "", nil).typ("new").typ("text").cmd("undo", true).doc("")
	mkHist(t, "", nil).typ("newtext").cmd("undo", true).cmd("redo", true).doc("newtext")
	mkHist(t, "", nil).typ("new").typ("text").cmd("undo", true).cmd("redo", true).doc("newtext")
}

func TestRedoPutsTheCursorAfterTheChange(t *testing.T) {
	h := mkHist(t, "one\n\ntwo", nil)
	h.update(TransactionSpec{Changes: []ChangeSpec{ins(3, "!")}, Selection: SingleSelection(4, 4)})
	h.sel(len(h.s.doc), len(h.s.doc)).cmd("undo", true).cmd("redo", true).head(4)
}

func TestSelectionIsRestoredOnUndoRedoUndo(t *testing.T) {
	h := mkHist(t, "1\n2\n3", nil).sel(5, 5)
	h.update(TransactionSpec{Changes: []ChangeSpec{ins(5, ".")}, Selection: SingleSelection(6, 6)})
	h.sel(0, 0).cmd("undo", true).head(5)
	h.sel(0, 0).cmd("redo", true).head(6)
	h.sel(0, 0).cmd("undo", true).head(5)
}

func TestHistoryTracksSeveralLevels(t *testing.T) {
	h := mkHist(t, "one", nil).typ("new").typ("text").typeAt("some", 0).doc("someonenewtext")
	h.cmd("undo", true).doc("onenewtext").cmd("undo", true).doc("one")
	h.cmd("redo", true).doc("onenewtext").cmd("redo", true).doc("someonenewtext")
	h.cmd("undo", true).doc("onenewtext")
}

func TestANewEventStartsWhenNewGroupDelayElapses(t *testing.T) {
	conf := defaultHistory
	conf.newGroupDelay = time.Second
	at := func(ms int) time.Time { return time.UnixMilli(int64(ms)) }
	h := mkHist(t, "", &conf).timedType("a", at(1000)).timedType("b", at(1600)).depths(1, 0)
	h.timedType("c", at(2700)).depths(2, 0)
	h.cmd("undo", true).timedType("d", at(2800)).depths(2, 0)
}

func TestACustomJoinPredicate(t *testing.T) {
	conf := defaultHistory
	conf.joinToEvent = func(tr Transaction, adjacent bool) bool {
		if !adjacent {
			return false
		}
		space := false
		tr.Changes.IterChanges(func(_, _, _, _ int, text string) {
			space = space || strings.HasPrefix(text, " ")
		})
		return !space
	}
	h := mkHist(t, "", &conf)
	for _, ch := range "ab cd" {
		h.typ(string(ch))
	}
	h.doc("ab cd").cmd("undo", true).doc("ab").cmd("undo", true).doc("")
}

func TestChangesCanStayOutOfTheHistory(t *testing.T) {
	mkHist(t, "", nil).typ("hello").receive("oops", 0, 0).receive("!", 9, 9).cmd("undo", true).doc("oops!")
	h := mkHist(t, "ab", nil).typeAt("cd", 1).receive("123", 0, 4)
	h.cmd("undo", false).cmd("redo", false)
}

func TestChangesMapThroughEachOtherAccurately(t *testing.T) {
	h := mkHist(t, "123", nil)
	h.update(TransactionSpec{Changes: []ChangeSpec{{From: 0, To: 1, Insert: "ab"}, {From: 1, To: 2, Insert: "cd"}, {From: 2, To: 3, Insert: "ef"}}})
	h.receive("!!!!!!!!", 2, 2).cmd("undo", true).cmd("redo", true).doc("ab!!!!!!!!cdef")
}

func TestComplexEditingSequences(t *testing.T) {
	h := mkHist(t, "", nil).typ("hello").isolate(IsolateBefore).typ("!").receive("....", 0, 0).typeAt("\n\n", 2)
	h.doc("..\n\n..hello!").receive("\n\n", 1, 1)
	h.cmd("undo", true).cmd("undo", true).doc(".\n\n...hello").cmd("undo", true).doc(".\n\n...")
}

func TestOverlappingEdits(t *testing.T) {
	h := mkHist(t, "", nil).typ("hello").isolate(IsolateBefore)
	h.update(TransactionSpec{Changes: []ChangeSpec{del(0, 5)}}).doc("")
	h.cmd("undo", true).doc("hello").cmd("undo", true).doc("")

	h = mkHist(t, "", nil).receive("h", 0, 0).typ("ello").isolate(IsolateBefore)
	h.update(TransactionSpec{Changes: []ChangeSpec{del(0, 5)}}).doc("")
	h.cmd("undo", true).doc("hello").cmd("undo", true).doc("h")

	h = mkHist(t, "", nil).typ("hi").isolate(IsolateBefore).typ("hello")
	h.update(TransactionSpec{Changes: []ChangeSpec{del(0, 7)}, SkipHistory: true}).doc("")
	h.cmd("undo", false).doc("")
}

func TestBackAndForthThroughHistory(t *testing.T) {
	h := mkHist(t, "", nil).typ("one").typ(" two").isolate(IsolateBefore).typ(" three").typeAt("zero ", 0)
	h.isolate(IsolateBefore).typeAt("\n\n", 0).typeAt("top", 0)
	for i := range 6 {
		for range 4 {
			h.cmd(map[bool]string{false: "undo", true: "redo"}[i%2 == 1], true)
		}
		h.doc(map[bool]string{false: "", true: "top\n\nzero one two three"}[i%2 == 1])
	}
	mkHist(t, "", nil).typ("o").typeAt("\n\n", 0).receive("zzz", 3, 3).cmd("undo", true).doc("zzz")

	h = mkHist(t, "", nil).typ("one").typ(" two").isolate(IsolateBefore)
	h.receive("xxx", len(h.s.doc), len(h.s.doc)).typ(" three").typeAt("zero ", 0).isolate(IsolateBefore)
	h.typeAt("\n\n", 0).typeAt("top", 0).receive("yyy", 0, 0)
	for range 3 {
		for range 4 {
			h.cmd("undo", true)
		}
		h.doc("yyyxxx")
		for range 4 {
			h.cmd("redo", true)
		}
		h.doc("yyytop\n\nzero one twoxxx three")
	}
}

func TestSelectionIsRestoredOnUndoAndRedo(t *testing.T) {
	h := mkHist(t, "", nil).typ("hi").isolate(IsolateBefore).sel(0, 2)
	before := h.s.sel
	h.update(TransactionSpec{Changes: []ChangeSpec{del(0, 2), ins(0, "hello")}})
	after := h.s.sel
	if h.cmd("undo", true); !h.s.sel.eq(before, false) {
		t.Fatalf("undo restored %s", rangesOf(h.s.sel))
	}
	if h.cmd("redo", true); !h.s.sel.eq(after, false) {
		t.Fatalf("redo restored %s", rangesOf(h.s.sel))
	}

	h = mkHist(t, "", nil)
	h.update(TransactionSpec{Changes: []ChangeSpec{ins(0, "a")}, Selection: SingleSelection(1, 1)})
	h.update(TransactionSpec{Changes: []ChangeSpec{ins(1, "b")}, Selection: SingleSelection(2, 2)})
	h.cmd("undo", true).doc("")
	if h.s.sel.Main().Anchor() != 0 {
		t.Fatal("undo did not restore the selection before the event's first change")
	}
}

func TestASelectionChangeInBetweenKeepsChangesApart(t *testing.T) {
	h := mkHist(t, "", nil).typ("hi").sel(0, 2)
	h.update(h.s.ReplaceSelection("hello")).depths(2, 0)
}

func TestUndoRebasesTheSelection(t *testing.T) {
	h := mkHist(t, "", nil).typ("hi").isolate(IsolateBefore).sel(0, 2)
	h.typeAt("hello", 0).receive("---", 0, 0).cmd("undo", true)
	if got := h.s.sel.ranges[0].Head(); got != 5 {
		t.Fatalf("head %d, want 5", got)
	}
}

func TestUndoAndRedoDepth(t *testing.T) {
	h := mkHist(t, "", nil).typ("a").depths(1, 0).receive("b", 0, 0).depths(1, 0)
	h.cmd("undo", true).depths(0, 1).cmd("redo", true).depths(1, 0)
}

func TestHistoryIsTruncated(t *testing.T) {
	conf := defaultHistory
	conf.minDepth = 10
	h := mkHist(t, "", &conf)
	for range 40 {
		h.typ("a").isolate(IsolateBefore)
	}
	if d := h.s.hist.depth(doneBranch); d >= 40 {
		t.Fatalf("depth %d after 40 isolated events with minDepth 10", d)
	}
}

func TestSelectionOnlyTransactionsAreNotUndone(t *testing.T) {
	mkHist(t, "abc", nil).head(0).sel(2, 2).cmd("undo", false).head(2)
}

func TestTransactionsAreIsolatedWhenAsked(t *testing.T) {
	h := mkHist(t, "", nil)
	for _, c := range []struct {
		at  int
		ch  string
		iso IsolateHistory
	}{{0, "a", IsolateAfter}, {1, "a", 0}, {2, "c", IsolateAfter}, {3, "d", 0}, {4, "e", IsolateFull}, {5, "f", 0}} {
		h.update(TransactionSpec{Changes: []ChangeSpec{ins(c.at, c.ch)}, IsolateHistory: c.iso})
	}
	h.depths(5, 0)
}

func TestEventsGroupAroundANonHistoryTransaction(t *testing.T) {
	h := mkHist(t, "", nil).typeAt("a", 0).receive("b", 1, 1).typeAt("c", 1)
	h.cmd("undo", true).doc("b")
}

func TestSelectionsMapThroughNonHistoryChanges(t *testing.T) {
	h := mkHist(t, "abc", nil)
	h.update(TransactionSpec{Selection: NewSelection([]SelectionRange{Cursor(0), Cursor(1), Cursor(2)}, 0)})
	h.update(TransactionSpec{Changes: []ChangeSpec{{From: 0, To: 3, Insert: "d"}}})
	h.update(TransactionSpec{Changes: []ChangeSpec{ins(0, "x"), ins(1, "y")}, SkipHistory: true})
	h.cmd("undo", true).doc("xabcy")
	var froms []string
	for _, r := range h.s.sel.ranges {
		froms = append(froms, fmt.Sprint(r.From()))
	}
	if got := strings.Join(froms, ","); got != "0,2,3" {
		t.Fatalf("ranges from %s, want 0,2,3", got)
	}
}

func TestSelectionIsRestoredOnRedo(t *testing.T) {
	h := mkHist(t, "a\nb\nc\n", nil)
	h.update(TransactionSpec{Selection: NewSelection([]SelectionRange{Cursor(1), Cursor(3), Cursor(5)}, 0)})
	h.update(h.s.ReplaceSelection("-")).sel(0, 0).cmd("undo", true).sel(0, 0).cmd("redo", true)
	var heads []string
	for _, r := range h.s.sel.ranges {
		heads = append(heads, fmt.Sprint(r.Head()))
	}
	if got := strings.Join(heads, ","); got != "2,5,8" {
		t.Fatalf("heads %s, want 2,5,8", got)
	}
}

func (h *histTest) selEvent(sel EditorSelection, event string) *histTest {
	h.t.Helper()
	return h.update(TransactionSpec{Selection: sel, UserEvent: event})
}

func TestUndoSelection(t *testing.T) {
	mkHist(t, "", nil).typ("newtext").cmd("undoSelection", true).doc("")
	mkHist(t, "abc", nil).sel(2, 2).cmd("undoSelection", true).head(0)
	// merges selection-only transactions from keyboard
	h := mkHist(t, "abc", nil)
	h.selEvent(SingleSelection(2, 2), "select").selEvent(SingleSelection(3, 3), "select").selEvent(SingleSelection(1, 1), "select")
	h.cmd("undoSelection", true).head(0)
	// doesn't merge selection-only transactions from other sources
	h = mkHist(t, "abc", nil).sel(2, 2).sel(3, 3).sel(1, 1)
	h.cmd("undoSelection", true).head(3).cmd("undoSelection", true).head(2).cmd("undoSelection", true).head(0)
	// doesn't merge when the number of selections changes
	h = mkHist(t, "abc", nil).selEvent(SingleSelection(2, 2), "select")
	h.selEvent(NewSelection([]SelectionRange{Cursor(1), Cursor(3)}, 0), "select").selEvent(SingleSelection(1, 1), "select")
	if h.cmd("undoSelection", true); len(h.s.sel.ranges) != 2 {
		t.Fatalf("%d ranges, want 2", len(h.s.sel.ranges))
	}
	h.cmd("undoSelection", true).head(0)
	// doesn't merge when a selection changes empty state
	h = mkHist(t, "abc", nil).selEvent(SingleSelection(2, 2), "select").selEvent(SingleSelection(2, 3), "select")
	h.selEvent(SingleSelection(1, 1), "select")
	if h.cmd("undoSelection", true); h.s.sel.Main().Anchor() != 2 || h.s.sel.Main().Head() != 3 {
		t.Fatalf("selection %s, want 2/3", rangesOf(h.s.sel))
	}
	h.cmd("undoSelection", true).head(0)
	// redo
	mkHist(t, "", nil).typ("newtext").cmd("undoSelection", true).cmd("redoSelection", true).doc("newtext")
	mkHist(t, "abc", nil).sel(2, 2).cmd("undoSelection", true).cmd("redoSelection", true).head(2)
}

func TestUndoSelectionOnlyChangesTheSelection(t *testing.T) {
	h := mkHist(t, "", nil).typ("hi").isolate(IsolateBefore)
	sel1 := h.s.sel
	h.sel(0, 2)
	sel2 := h.s.sel
	if h.cmd("undoSelection", true); !h.s.sel.eq(sel1, false) || h.s.doc != "hi" {
		t.Fatalf("undoSelection left %s %q", rangesOf(h.s.sel), h.s.doc)
	}
	if h.cmd("redoSelection", true); !h.s.sel.eq(sel2, false) {
		t.Fatalf("redoSelection left %s", rangesOf(h.s.sel))
	}
	h.update(h.s.ReplaceSelection("hello"))
	if h.cmd("undoSelection", true); !h.s.sel.eq(sel2, false) {
		t.Fatalf("undoSelection after a change left %s", rangesOf(h.s.sel))
	}
	h.cmd("redo", true).head(5)
}

func TestUndoSelectionThroughRemoteChanges(t *testing.T) {
	h := mkHist(t, "", nil).typ("hello")
	sel := h.s.sel
	h.sel(0, 2).receive("oops", 0, 0).receive("!", 9, 9)
	if !h.s.sel.eq(SingleSelection(4, 6), false) {
		t.Fatalf("selection %s, want 4/6", rangesOf(h.s.sel))
	}
	h.cmd("undoSelection", true).doc("oopshello!")
	if !h.s.sel.eq(sel, false) {
		t.Fatalf("selection %s, want %s", rangesOf(h.s.sel), rangesOf(sel))
	}
	mkHist(t, "", nil).typ("1234").update(TransactionSpec{Changes: []ChangeSpec{ins(2, "x")}, SkipHistory: true}).cmd("undo", true).doc("x")
}

func TestCtrlZUndoesTypingAsOneEventAndCtrlYRedoesIt(t *testing.T) {
	h, events := focusedEditor(t, EditorStateConfig{Doc: ""}, SingleSelection(0, 0))
	for _, ch := range "abc" {
		h.typeText(string(ch))
		h.now = h.now.Add(100 * time.Millisecond)
	}
	h.now = h.now.Add(time.Second)
	h.typeText(" d")
	h.press("Z", key.ModShortcut)
	if got := h.ed.State().Doc(); got != "abc" {
		t.Fatalf("Ctrl+Z left %q, want the last word undone alone", got)
	}
	h.press("Z", key.ModShortcut)
	if got := h.ed.State().Doc(); got != "" || h.ed.UndoDepth() != 0 || h.ed.RedoDepth() != 2 {
		t.Fatalf("Ctrl+Z again left %q, depths %d/%d", got, h.ed.UndoDepth(), h.ed.RedoDepth())
	}
	h.press("Y", key.ModShortcut)
	h.press("Z", key.ModShortcut|key.ModShift)
	if got := h.ed.State().Doc(); got != "abc d" {
		t.Fatalf("Ctrl+Y and Ctrl+Shift+Z left %q", got)
	}
	if !strings.Contains(strings.Join(*events, " "), "undo") || !strings.Contains(strings.Join(*events, " "), "redo") {
		t.Errorf("events %q, want undo and redo", *events)
	}
}

func TestAnIsolatedDispatchIsItsOwnUndoEvent(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "x"}, SingleSelection(1, 1))
	h.typeText("y")
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(2, "OURS")}, IsolateHistory: IsolateFull})
	h.frame()
	if !h.ed.Undo() || h.ed.State().Doc() != "xy" {
		t.Fatalf("Undo left %q, want the take-ours change alone undone", h.ed.State().Doc())
	}
	if !h.ed.Redo() || h.ed.State().Doc() != "xyOURS" {
		t.Fatalf("Redo left %q", h.ed.State().Doc())
	}
}

func TestSelectionsOfAnotherUserEventAreNotMerged(t *testing.T) {
	// What @codemirror/commands 6.11.1 gives, run in node: a "select" then
	// two "select.pointer" are two selection events, the second merged.
	h := mkHist(t, "abc", nil)
	h.selEvent(SingleSelection(2, 2), "select").selEvent(SingleSelection(3, 3), "select.pointer")
	h.selEvent(SingleSelection(1, 1), "select.pointer")
	h.cmd("undoSelection", true).head(2).cmd("undoSelection", true).head(0)
}
