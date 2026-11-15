package godemirror

import (
	"fmt"
	"strings"
	"testing"
)

// The selection tests are @codemirror/state 6.7.6's test/test-selection.ts,
// and the state tests the cases of its test/test-state.ts that touch what is
// ported (the document, the selection, changeByRange and the line
// separator), ported and run through an Editor.

func rangesOf(s EditorSelection) string {
	var parts []string
	for _, r := range s.ranges {
		parts = append(parts, fmt.Sprintf("%d/%d", r.Anchor(), r.Head()))
	}
	return strings.Join(parts, ",")
}

func TestSelectionStoresRangesWithAMainRange(t *testing.T) {
	sel := NewSelection([]SelectionRange{Range(0, 1), Range(3, 2), Range(4, 5)}, 1)
	m := sel.Main()
	if m.From() != 2 || m.To() != 3 || m.Anchor() != 3 || m.Head() != 2 {
		t.Errorf("main = %d-%d anchor %d head %d, want 2-3 anchor 3 head 2", m.From(), m.To(), m.Anchor(), m.Head())
	}
	if got := rangesOf(sel); got != "0/1,3/2,4/5" {
		t.Errorf("ranges %s", got)
	}
}

func TestSelectionMergesAndSortsRanges(t *testing.T) {
	sel := NewSelection([]SelectionRange{
		Range(10, 12), Range(6, 7), Range(4, 5), Range(3, 4), Range(0, 6), Range(7, 8), Range(9, 13), Range(13, 14),
	}, 0)
	if got := rangesOf(sel); got != "0/6,6/7,7/8,9/13,13/14" {
		t.Errorf("ranges %s", got)
	}
}

func TestSelectionMergesAdjacentCursors(t *testing.T) {
	sel := NewSelection([]SelectionRange{Range(10, 12), Range(12, 12), Range(12, 12), Range(10, 10), Range(8, 10)}, 0)
	if got := rangesOf(sel); got != "8/10,10/12" {
		t.Errorf("ranges %s", got)
	}
}

func TestSelectionMergingKeepsTheDirectionOfTheLaterRange(t *testing.T) {
	if got := rangesOf(NewSelection([]SelectionRange{Range(0, 2), Range(10, 1)}, 0)); got != "10/0" {
		t.Errorf("ranges %s", got)
	}
}

func TestSelectionMergingFollowsTheMainRange(t *testing.T) {
	sel := NewSelection([]SelectionRange{Range(8, 9), Range(0, 2), Range(1, 4)}, 2)
	if got := rangesOf(sel); got != "0/4,8/9" || sel.MainIndex() != 0 {
		t.Errorf("ranges %s main %d, want 0/4,8/9 main 0", got, sel.MainIndex())
	}
}

func TestExtendKeepsTheAnchorUnlessItFallsInside(t *testing.T) {
	for _, c := range []struct {
		r        SelectionRange
		from, to int
		want     string
	}{
		{Range(5, 8), 10, 10, "5/10"},
		{Range(5, 8), 2, 2, "5/2"},
		{Range(5, 8), 2, 12, "2/12"},
		{Range(8, 5), 3, 6, "8/3"},
		{Cursor(4), 1, 9, "1/9"},
	} {
		if got := rangesOf(SingleSelection(0, 0).ReplaceRange(c.r.Extend(c.from, c.to), 0)); got != c.want {
			t.Errorf("%d/%d extended to %d-%d = %s, want %s", c.r.Anchor(), c.r.Head(), c.from, c.to, got, c.want)
		}
	}
}

func newTestEditor(t *testing.T, c EditorStateConfig) *Editor {
	t.Helper()
	e, err := NewEditor("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetState(c); err != nil {
		t.Fatal(err)
	}
	return e
}

func dispatch(t *testing.T, e *Editor, spec TransactionSpec) {
	t.Helper()
	if err := e.Dispatch(spec); err != nil {
		t.Fatal(err)
	}
}

func TestEditorStateHoldsDocAndSelection(t *testing.T) {
	s := newTestEditor(t, EditorStateConfig{Doc: "hello"}).State()
	if s.Doc() != "hello" || s.Selection().Main().From() != 0 {
		t.Errorf("doc %q, selection at %d", s.Doc(), s.Selection().Main().From())
	}
}

func TestEditorAppliesChanges(t *testing.T) {
	e := newTestEditor(t, EditorStateConfig{Doc: "hello"})
	dispatch(t, e, TransactionSpec{Changes: []ChangeSpec{{From: 2, To: 4, Insert: "w"}, ins(5, "!")}})
	if got := e.State().Doc(); got != "hewo!" {
		t.Errorf("doc %q, want hewo!", got)
	}
}

func TestReplaceSelectionMapsEveryRange(t *testing.T) {
	e := newTestEditor(t, EditorStateConfig{
		Doc:                     "abcdefgh",
		AllowMultipleSelections: true,
		Selection:               NewSelection([]SelectionRange{Cursor(0), Cursor(4), Cursor(8)}, 0),
	})
	dispatch(t, e, e.State().ReplaceSelection("Q"))
	s := e.State()
	if s.Doc() != "QabcdQefghQ" || rangesOf(s.Selection()) != "1/1,6/6,11/11" {
		t.Errorf("doc %q, ranges %s; want QabcdQefghQ, 1/1,6/6,11/11", s.Doc(), rangesOf(s.Selection()))
	}
}

func TestDispatchRefusesInvalidChanges(t *testing.T) {
	e := newTestEditor(t, EditorStateConfig{Doc: "1234é"})
	for _, spec := range []TransactionSpec{
		{Changes: []ChangeSpec{del(-1, 1)}},
		{Changes: []ChangeSpec{del(2, 1)}},
		{Changes: []ChangeSpec{{From: 2, To: 10, Insert: "x"}}},
		{Changes: []ChangeSpec{ins(5, "x")}},
		{Selection: SingleSelection(0, 7)},
		{Selection: SingleSelection(5, 5)},
	} {
		if err := e.Dispatch(spec); err == nil {
			t.Errorf("%+v was dispatched", spec)
		}
	}
	if e.State().Doc() != "1234é" {
		t.Errorf("doc changed to %q", e.State().Doc())
	}
}

func TestSingleSelectionsUnlessMultipleAreAllowed(t *testing.T) {
	cursors := NewSelection([]SelectionRange{Cursor(0), Cursor(1)}, 0)
	e := newTestEditor(t, EditorStateConfig{Doc: "123", Selection: cursors})
	if n := len(e.State().Selection().ranges); n != 1 {
		t.Errorf("created with %d ranges", n)
	}
	dispatch(t, e, TransactionSpec{Selection: cursors})
	if n := len(e.State().Selection().ranges); n != 1 {
		t.Errorf("updated to %d ranges", n)
	}
}

func TestChangeByRangeMakesSimpleChanges(t *testing.T) {
	e := newTestEditor(t, EditorStateConfig{Doc: "hi"})
	dispatch(t, e, e.State().ChangeByRange(func(r SelectionRange) RangeChange {
		return RangeChange{Changes: []ChangeSpec{{From: r.From(), To: r.From() + 1, Insert: "q"}}, Range: Cursor(r.From() + 1)}
	}))
	if s := e.State(); s.Doc() != "qi" || s.Selection().Main().From() != 1 {
		t.Errorf("doc %q, selection at %d", s.Doc(), s.Selection().Main().From())
	}
}

func TestChangeByRangeMapsEachRangeThroughTheOthers(t *testing.T) {
	e := newTestEditor(t, EditorStateConfig{
		Doc:                     "1 2 3 4",
		AllowMultipleSelections: true,
		Selection:               NewSelection([]SelectionRange{Range(0, 1), Range(2, 3), Range(4, 5), Range(6, 7)}, 0),
	})
	dispatch(t, e, e.State().ChangeByRange(func(r SelectionRange) RangeChange {
		n := r.From() >> 1
		return RangeChange{
			Changes: []ChangeSpec{{From: r.From(), To: r.To(), Insert: strings.Repeat("-", n+1)}},
			Range:   Range(r.From(), r.From()+1+n),
		}
	}))
	s := e.State()
	var got []string
	for _, r := range s.Selection().Ranges() {
		got = append(got, fmt.Sprintf("%d-%d", r.From(), r.To()))
	}
	if s.Doc() != "- -- --- ----" || strings.Join(got, " ") != "0-1 2-4 5-8 9-13" {
		t.Errorf("doc %q, ranges %v", s.Doc(), got)
	}
}

func TestTheLineSeparatorSplitsTheDocAndJoinsSlices(t *testing.T) {
	plainLF := newTestEditor(t, EditorStateConfig{Doc: "a\r\nb\rc\n"}).State()
	if plainLF.Doc() != "a\nb\nc\n" || plainLF.Lines() != 4 {
		t.Errorf("default separator: doc %q, %d lines", plainLF.Doc(), plainLF.Lines())
	}
	e := newTestEditor(t, EditorStateConfig{Doc: "a\r\nb\rc", LineSeparator: "\r\n"})
	dispatch(t, e, TransactionSpec{Changes: []ChangeSpec{ins(1, "x\r\ny")}})
	s := e.State()
	if s.Doc() != "ax\ny\nb\rc" || s.Lines() != 3 {
		t.Errorf("CRLF: doc %q, %d lines", s.Doc(), s.Lines())
	}
	if got := s.SliceDoc(0, len(s.Doc())); got != "ax\r\ny\r\nb\rc" {
		t.Errorf("CRLF slice %q", got)
	}
}

func TestLinesAreNumberedFromOneAndEndBeforeTheirBreak(t *testing.T) {
	s := newTestEditor(t, EditorStateConfig{Doc: "ab\n\ncd\n"}).State()
	if s.Lines() != 4 {
		t.Fatalf("%d lines, want 4", s.Lines())
	}
	for pos, want := range map[int]Line{
		0: {1, 0, 2, "ab"}, 2: {1, 0, 2, "ab"}, 3: {2, 3, 3, ""}, 5: {3, 4, 6, "cd"}, 7: {4, 7, 7, ""},
	} {
		if got := s.LineAt(pos); got != want {
			t.Errorf("LineAt(%d) = %+v, want %+v", pos, got, want)
		}
	}
}
