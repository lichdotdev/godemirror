package main

import (
	"slices"
	"strconv"
	"testing"
)

const patch = "diff --git a/main.go b/main.go\n" +
	"index 1111111..2222222 100644\n" +
	"--- a/main.go\n" +
	"+++ b/main.go\n" +
	"@@ -3,4 +3,3 @@ package main\n" +
	" a\n" +
	"-b\n" +
	"-b2\n" +
	"+B\n" +
	" e\n" +
	"@@ -10 +9,2 @@\n" +
	" c\n" +
	"+d\n" +
	"\\ No newline at end of file\n" +
	"diff --git a/other.go b/other.go\n" +
	"+++ b/other.go\n" +
	"@@ -1 +1 @@\n" +
	"+ignored\n"

// newFile is main.go after the change: line n reads "n<n>" outside the hunks.
func newFile() []string {
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = "n" + strconv.Itoa(i+1)
	}
	return lines
}

func model(t *testing.T, withText bool) *diffModel {
	t.Helper()
	f, err := parseFileDiff(patch)
	if err != nil {
		t.Fatal(err)
	}
	if f.path != "main.go" || len(f.hunks) != 2 {
		t.Fatalf("parsed %q with %d hunks", f.path, len(f.hunks))
	}
	m := &diffModel{file: f, expanded: map[int]bool{}, reverted: map[int]bool{}}
	if withText {
		m.newText = newFile()
	}
	return m
}

func kinds(lines []diffLine) []lineKind {
	out := make([]lineKind, len(lines))
	for i, l := range lines {
		out[i] = l.kind
	}
	return out
}

func TestUnifiedLaysOutLikeTheWebDiff(t *testing.T) {
	u := unified(model(t, false).lines())
	if want := "\na\nb\nb2\nB\ne\n\nc\nd"; u.text != want {
		t.Fatalf("text = %q, want %q", u.text, want)
	}
	if want := []string{"", "3", "4", "5", "4", "5", "", "9", "10"}; !slices.Equal(u.labels, want) {
		t.Fatalf("labels = %q, want %q", u.labels, want)
	}
	if len(u.gaps) != 0 {
		t.Fatalf("gaps offered with no new text to pull them from: %v", u.gaps)
	}
	if want := map[int]int{1: 2, 2: 8}; len(u.reverts) != 2 || u.reverts[1] != want[1] || u.reverts[2] != want[2] {
		t.Fatalf("revert buttons on %v, want %v", u.reverts, want)
	}
}

func TestSplitPairsChangesAndPadsWithFillers(t *testing.T) {
	left, right := split(model(t, false).lines())
	if want := "\na\nb\nb2\ne\n\nc\n"; left.text != want {
		t.Fatalf("left = %q, want %q", left.text, want)
	}
	if want := "\na\nB\n\ne\n\nc\nd"; right.text != want {
		t.Fatalf("right = %q, want %q", right.text, want)
	}
	if want := []string{"", "3", "4", "5", "6", "", "10", ""}; !slices.Equal(left.labels, want) {
		t.Fatalf("left labels = %q, want %q", left.labels, want)
	}
	if want := []string{"", "3", "4", "", "5", "", "9", "10"}; !slices.Equal(right.labels, want) {
		t.Fatalf("right labels = %q, want %q", right.labels, want)
	}
	if left.decorations[3].Hatch.A != 0 || right.decorations[3].Hatch.A == 0 {
		t.Fatal("filler hatch on the wrong side")
	}
	if right.decorations[3].HoverGroup != left.decorations[3].HoverGroup || right.decorations[3].HoverGroup == 0 {
		t.Fatal("a filler must belong to its block, on both sides")
	}
	if len(left.reverts) != 0 || len(right.reverts) != 2 {
		t.Fatalf("revert buttons: left %v, right %v, want only the right's", left.reverts, right.reverts)
	}
}

func TestExpandingAGapPullsItsLinesIn(t *testing.T) {
	m := model(t, true)
	before := unified(m.lines())
	if len(before.gaps) != 2 {
		t.Fatalf("gaps offered on lines %v, want both separators", before.gaps)
	}
	m.expanded[1] = true
	lines := m.lines()
	want := []lineKind{lineContext, lineContext, lineContext, lineDel, lineDel, lineAdd, lineContext, lineSeparator, lineContext, lineAdd}
	if got := kinds(lines); !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	if lines[0].text != "n1" || lines[1].text != "n2" || lines[1].oldLine != 2 || lines[1].newLine != 2 {
		t.Fatalf("pulled lines = %+v %+v", lines[0], lines[1])
	}
}

func TestRevertingABlockRestoresItAndRenumbersBelow(t *testing.T) {
	m := model(t, false)
	m.reverted[1] = true
	u := unified(m.lines())
	if want := "\na\nb\nb2\ne\n\nc\nd"; u.text != want {
		t.Fatalf("text = %q, want %q", u.text, want)
	}
	// b and b2 are the new file's lines 4 and 5 again, so e moves to 6 and
	// the next hunk down by one.
	if want := []string{"", "3", "4", "5", "6", "", "10", "11"}; !slices.Equal(u.labels, want) {
		t.Fatalf("labels = %q, want %q", u.labels, want)
	}
	if _, ok := u.reverts[1]; ok || len(u.reverts) != 1 {
		t.Fatalf("revert buttons = %v, want only block 2's", u.reverts)
	}
}

// idOf finds the line reading text and returns its id.
func idOf(t *testing.T, lines []diffLine, text string) int {
	t.Helper()
	for _, l := range lines {
		if l.text == text {
			return l.id
		}
	}
	t.Fatalf("no line %q", text)
	return 0
}

func TestLineIDsSurviveExpandingAndReverting(t *testing.T) {
	m := model(t, true)
	before := m.lines()
	e, d := idOf(t, before, "e"), idOf(t, before, "d")
	b := idOf(t, before, "b")
	m.expanded[1] = true
	m.reverted[1] = true
	after := m.lines()
	if idOf(t, after, "e") != e || idOf(t, after, "d") != d || idOf(t, after, "b") != b {
		t.Fatal("a line's id moved when the diff around it was rebuilt")
	}
	if idOf(t, after, "n1") >= 0 {
		t.Fatal("a line pulled out of a gap must not share the hunk lines' ids")
	}
}
