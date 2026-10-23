package godemirror

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/lichdotdev/godemirror/tags"
)

func newDiffHarness(t *testing.T, opts DiffOptions) (*harness, *DiffView) {
	t.Helper()
	d, err := NewDiffView("monospace", 12, opts)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, v: d.unified, diff: d, size: image.Pt(800, 300), now: time.Unix(1, 0)}
	h.frame()
	return h, d
}

func TestDiffViewShowsTwoDocsUnifiedWithBothNumbers(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetDocs("main.go", "a\nb\nc\n", "a\nB\nc\n")
	h.frame()
	var texts, labels []string
	for i := range h.v.LineCount() {
		texts, labels = append(texts, h.v.LineText(i)), append(labels, h.v.label(i))
	}
	if got, want := strings.Join(texts, "|"), "a|b|B|c"; got != want {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	if got, want := strings.Join(labels, "|"), "1 1|2  |  2|3 3"; got != want {
		t.Fatalf("labels = %q, want %q", got, want)
	}
}

func TestDiffViewColorsChangesFromItsTheme(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetDocs("main.go", "a\nb\nc", "a\nB\nc")
	h.frame()
	for _, theme := range []Theme{LightTheme, DarkTheme} {
		d.SetTheme(theme)
		h.frame()
		if got := h.v.decoration(1).Background; got != theme.Diff.Deleted.Background {
			t.Fatalf("deleted row background = %v, want %v", got, theme.Diff.Deleted.Background)
		}
		if got := h.v.decoration(2).GutterStrip; got != theme.Diff.Added.Strip {
			t.Fatalf("added row strip = %v, want %v", got, theme.Diff.Added.Strip)
		}
		if h.v.decoration(1).HoverGroup == 0 || h.v.decoration(1).HoverGroup != h.v.decoration(2).HoverGroup {
			t.Fatal("a chunk's rows do not light up together")
		}
	}
}

func TestDiffViewSplitPairsChangesSideBySide(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetDocs("main.go", "a\nb\nb2\nc", "a\nB\nc")
	d.Split = true
	h.frame()
	left, right := d.split.Left, d.split.Right
	if left.LineCount() != 4 || right.LineCount() != 4 {
		t.Fatalf("sides have %d and %d rows, want 4 each", left.LineCount(), right.LineCount())
	}
	if left.LineText(2) != "b2" || right.LineText(2) != "" || right.decoration(2).Hatch != DarkTheme.Diff.Filler {
		t.Fatalf("row 2: %q | %q, want b2 paired with a hatched filler", left.LineText(2), right.LineText(2))
	}
	if left.label(1) != "2" || right.label(1) != "2" || right.label(2) != "" {
		t.Fatalf("labels %q | %q, %q", left.label(1), right.label(1), right.label(2))
	}
}

func TestDiffViewKeepsItsPlaceForANewVersionOfTheSameFile(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	old := numbered(100)
	d.SetDocs("main.go", old, strings.Replace(old, "line 50", "LINE 50", 1))
	h.frame()
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 100), Scroll: f32.Pt(0, 300)})
	d.SetDocs("main.go", old, strings.Replace(old, "line 60", "LINE 60", 1))
	waitForHighlight(h)
	if h.v.scroll.Y != 300 {
		t.Fatalf("scroll = %d after a new version of the file, want 300", h.v.scroll.Y)
	}
	d.SetDocs("other.go", old, old+"\nmore")
	h.frame()
	if h.v.scroll.Y != 0 {
		t.Fatalf("scroll = %d after another file, want 0", h.v.scroll.Y)
	}
}

func TestDiffViewShowsAFileOfAPatch(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetPatch(parsePatch(t, twoFilePatch)[0])
	h.frame()
	if got := h.v.LineCount(); got != 9 {
		t.Fatalf("%d rows, want 9", got)
	}
	if chunks := d.Chunks(); len(chunks) != 2 || chunks[0].Lines[0] != (ChangedLine{OldSide, 4, "b"}) {
		t.Fatalf("Chunks() = %+v", chunks)
	}
}

// commentAcrossGapPatch opens a block comment in its first hunk and closes
// it in the lines its gap leaves out; a later comment's end is the first
// "*/" the text on screen holds.
const commentAcrossGapPatch = `diff --git a/A.java b/A.java
--- a/A.java
+++ b/A.java
@@ -1,2 +1,2 @@
-class A {
+class B {
   /**
@@ -6,3 +6,3 @@
-  String s = "x";
+  String s = "y";
   /** later */
 }
`

func TestAGapEndsWhatItsHunkLeftOpen(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetPatch(parsePatch(t, commentAcrossGapPatch)[0])
	waitForHighlight(h)
	if got := tagAt(t, h.v, 4, `"x"`); got != tags.String {
		t.Fatalf(`unified: "x" after the gap is %v, want %v`, got, tags.String)
	}
	d.Split = true
	h.v = d.split.Right
	h.frame()
	waitForHighlight(h)
	if got := tagAt(t, h.v, 3, `"y"`); got != tags.String {
		t.Fatalf(`split: "y" after the gap is %v, want %v`, got, tags.String)
	}
}

func TestNewDiffViewFromFontsNeedsNoFontconfig(t *testing.T) {
	withoutFontconfig(t)
	d, err := NewDiffViewFromFonts(goMono, 12, DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	d.SetDocs("a.txt", "x", "y")
	h := &harness{t: t, v: d.unified, diff: d, size: image.Pt(400, 200), now: time.Unix(1, 0)}
	h.frame()
}

// waitForHighlight draws frames until the view has taken its highlighting in.
func waitForHighlight(h *harness) {
	h.t.Helper()
	for deadline := time.Now().Add(5 * time.Second); h.v.pending != nil; {
		if time.Now().After(deadline) {
			h.t.Fatal("highlighting never landed")
		}
		time.Sleep(time.Millisecond)
		h.frame()
	}
}

// collapsedDocs is a 20-line file with line 10 changed, collapsed so that
// lines 1..6 and 14..20 are left out.
func collapsedDocs(d *DiffView) {
	a := strings.Join(fileLines(20), "\n")
	d.SetDocs("main.go", a, strings.Replace(a, "l10\n", "L10\n", 1))
}

var defaultCollapse = &Collapse{Margin: 3, MinSize: 4}

func TestExpandingAGapOfTwoDocsFillsItIn(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{Collapse: defaultCollapse})
	collapsedDocs(d)
	h.frame()
	h.click(h.at(0, 3), 0)
	h.frame()
	waitForHighlight(h)
	if got := h.v.LineText(0); got != "l1" || h.v.LineCount() != 15 {
		t.Fatalf("after expanding the top gap: %d rows, first %q; want 15 rows from l1", h.v.LineCount(), got)
	}
}

func TestALastRowWithNoTextIsStillShown(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{Collapse: defaultCollapse})
	collapsedDocs(d)
	h.frame()
	if got := h.v.LineCount(); got != 10 {
		t.Fatalf("%d rows, want 10: the separator for lines 14..20 is missing", got)
	}
	d.SetDocs("other.go", "a\nb\nc", "a\nB")
	d.Split = true
	h.frame()
	if l, r := d.split.Left.LineCount(), d.split.Right.LineCount(); l != 3 || r != 3 {
		t.Fatalf("sides have %d and %d rows, want 3 each: the last filler is missing", l, r)
	}
}

func TestAPatchGapIsFilledByTheCallerInParts(t *testing.T) {
	var asked []Gap
	h, d := newDiffHarness(t, DiffOptions{Expand: func(g Gap) { asked = append(asked, g) }})
	d.SetPatch(parsePatch(t, twoFilePatch)[0])
	h.frame()
	h.click(h.at(6, 3), 0)
	h.frame()
	if len(asked) != 1 || asked[0].From != 6 || asked[0].To != 8 || asked[0].OldFrom != 7 {
		t.Fatalf("asked for %+v, want one gap of new lines 6..8", asked)
	}
	g := asked[0]
	d.FillGap(g, []string{"n6"})
	g.From++
	d.FillGap(g, []string{"n7", "n8", "past the gap"})
	waitForHighlight(h)
	if got := h.v.LineText(6) + h.v.LineText(7) + h.v.LineText(8) + h.v.LineText(9); got != "n6n7n8c" {
		t.Fatalf("rows 6..9 read %q, want n6 n7 n8 then c", got)
	}
}

func TestAFillForAnOlderDiffIsDropped(t *testing.T) {
	var asked []Gap
	h, d := newDiffHarness(t, DiffOptions{Expand: func(g Gap) { asked = append(asked, g) }})
	file := parsePatch(t, twoFilePatch)[0]
	d.SetPatch(file)
	h.frame()
	h.click(h.at(6, 3), 0)
	h.frame()
	file.body += " "
	d.SetPatch(file)
	waitForHighlight(h)
	rows := h.v.LineCount()
	d.FillGap(asked[0], []string{"stale"})
	waitForHighlight(h)
	if h.v.LineCount() != rows {
		t.Fatal("lines asked for before the diff changed were laid into the new one")
	}
}

func TestRevertHandsTheChunkUnderThePointerToTheCaller(t *testing.T) {
	var reverted []Chunk
	h, d := newDiffHarness(t, DiffOptions{Revert: func(c Chunk) { reverted = append(reverted, c) }})
	d.SetPatch(parsePatch(t, twoFilePatch)[0])
	h.frame()
	row := h.at(2, 0)
	h.pointer(pointer.Move, row, 0)
	h.click(f32.Pt(float32(h.size.X)-12, row.Y), 0)
	h.frame()
	if len(reverted) != 1 || !chunkEqual(reverted[0], d.Chunks()[0]) {
		t.Fatalf("reverted %+v, want the first chunk", reverted)
	}
}

func TestTheExpandLabelCountsTheHiddenLines(t *testing.T) {
	if got := defaultExpandLabel(1); got != "1 unchanged line" {
		t.Fatalf("got %q", got)
	}
	if got := defaultExpandLabel(44); got != "44 unchanged lines" {
		t.Fatalf("got %q", got)
	}
	var hidden []int
	h, d := newDiffHarness(t, DiffOptions{Collapse: defaultCollapse, ExpandLabel: func(n int) string {
		hidden = append(hidden, n)
		return "more"
	}})
	collapsedDocs(d)
	h.frame()
	if !slices.Contains(hidden, 6) || !slices.Contains(hidden, 7) {
		t.Fatalf("ExpandLabel was asked for %v, want 6 and 7", hidden)
	}
}

// selectRows selects rows from to to of the view under test with the
// pointer: a triple click on the first, a Shift+click inside the last.
func selectRows(h *harness, from, to int, offsetX float32) {
	h.t.Helper()
	first := h.at(from, 1)
	first.X += offsetX
	h.now = h.now.Add(time.Second)
	for range 3 {
		h.click(first, 0)
	}
	last := h.at(to, 1)
	last.X += offsetX
	h.click(last, key.ModShift)
}

func TestSelectedRangeAndChangesNameFileLines(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetPatch(parsePatch(t, twoFilePatch)[0])
	h.frame()
	selectRows(h, 1, 4, 0)
	r, ok := d.SelectedRange()
	if want := (DiffRange{OldFrom: 3, OldTo: 5, NewFrom: 3, NewTo: 4}); !ok || r != want {
		t.Fatalf("SelectedRange() = %+v, %v; want %+v", r, ok, want)
	}
	want := []ChangedLine{{OldSide, 4, "b"}, {OldSide, 5, "b2"}, {NewSide, 4, "B"}}
	if got := d.SelectedChanges(); !slices.Equal(got, want) {
		t.Fatalf("SelectedChanges() = %+v, want %+v", got, want)
	}
}

func TestASelectionOnTheRightSideNamesOnlyNewLines(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetPatch(parsePatch(t, twoFilePatch)[0])
	d.Split = true
	h.v = d.split.Right
	h.frame()
	selectRows(h, 1, 3, float32(h.size.X/2))
	r, ok := d.SelectedRange()
	if want := (DiffRange{NewFrom: 3, NewTo: 4}); !ok || r != want {
		t.Fatalf("SelectedRange() = %+v, %v; want %+v", r, ok, want)
	}
	if got, want := d.SelectedChanges(), []ChangedLine{{NewSide, 4, "B"}}; !slices.Equal(got, want) {
		t.Fatalf("SelectedChanges() = %+v, want %+v", got, want)
	}
}

func TestNothingSelectedIsNoRange(t *testing.T) {
	_, d := newDiffHarness(t, DiffOptions{})
	d.SetPatch(parsePatch(t, twoFilePatch)[0])
	if _, ok := d.SelectedRange(); ok || d.SelectedChanges() != nil {
		t.Fatal("a range with nothing selected")
	}
}

func TestTheSelectionIsReadAgainstTheRowsOnScreen(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	file := parsePatch(t, twoFilePatch)[0]
	d.SetPatch(file)
	h.frame()
	selectRows(h, 2, 2, 0)
	file.hunks = file.hunks[1:]
	file.body += " "
	d.SetPatch(file)
	if got, want := d.SelectedChanges(), []ChangedLine{{OldSide, 4, "b"}}; !slices.Equal(got, want) {
		t.Fatalf("before the new version shows, SelectedChanges() = %+v, want %+v", got, want)
	}
}

func TestBlocksOpenUnderTheirFileLine(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetPatch(parsePatch(t, twoFilePatch)[0])
	h.frame()
	thread := func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(10, 40)} }
	d.SetBlocks([]DiffBlock{{At: LineRef{NewSide, 4}, Widget: thread}, {At: LineRef{OldSide, 5}, Widget: thread}, {At: LineRef{NewSide, 7}, Widget: thread}})
	waitForHighlight(h)
	after := func(v *View) []int {
		var out []int
		for _, b := range v.content.Blocks {
			out = append(out, b.After)
		}
		return out
	}
	if got := after(d.unified); !slices.Equal(got, []int{4, 3}) {
		t.Fatalf("unified blocks under rows %v, want [4 3]: B, then b2; line 7 is in a gap", got)
	}
	d.Split = true
	h.v = d.split.Left
	waitForHighlight(h)
	h.v = d.split.Right
	waitForHighlight(h)
	if l, r := after(d.split.Left), after(d.split.Right); !slices.Equal(l, []int{3}) || !slices.Equal(r, []int{2}) {
		t.Fatalf("split blocks under rows %v | %v, want [3] | [2]", l, r)
	}
}
