package godemirror

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

const twoFilePatch = "diff --git a/main.go b/main.go\n" +
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
	"diff --git a/gone.go b/gone.go\n" +
	"deleted file mode 100644\n" +
	"--- a/gone.go\n" +
	"+++ /dev/null\n" +
	"@@ -1 +0,0 @@\n" +
	"-bye\n"

func parsePatch(t *testing.T, patch string) []FilePatch {
	t.Helper()
	files, err := ParsePatch(patch)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestParsePatchReadsEveryFile(t *testing.T) {
	files := parsePatch(t, twoFilePatch)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if f := files[0]; f.Path != "main.go" || f.OldPath != "main.go" || f.Added != 2 || f.Deleted != 2 {
		t.Fatalf("first file = %+v", f)
	}
	if f := files[1]; f.Path != "gone.go" || f.Deleted != 1 {
		t.Fatalf("deleted file = %s -%d, want gone.go -1", f.Path, f.Deleted)
	}
}

func TestParsePatchRejectsABrokenHunkHeader(t *testing.T) {
	if _, err := ParsePatch("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -a +1 @@\n"); err == nil {
		t.Fatal("no error for a hunk header that is not numbers")
	}
}

func TestUnifiedNumbersBothColumnsAndOffersGaps(t *testing.T) {
	u := unified(parsePatch(t, twoFilePatch)[0].source().rows(nil))
	if want := "\na\nb\nb2\nB\ne\n\nc\nd"; u.text != want {
		t.Fatalf("text = %q, want %q", u.text, want)
	}
	want := []string{"", " 3  3", " 4   ", " 5   ", "    4", " 6  5", "", "10  9", "   10"}
	if !slices.Equal(u.labels, want) {
		t.Fatalf("labels = %q, want %q", u.labels, want)
	}
	if g := u.gaps[0]; g == nil || g.From != 1 || g.To != 2 || g.OldFrom != 1 {
		t.Fatalf("top gap = %+v", u.gaps[0])
	}
	if g := u.gaps[6]; g == nil || g.From != 6 || g.To != 8 || g.OldFrom != 7 {
		t.Fatalf("middle gap = %+v", u.gaps[6])
	}
}

func TestPullingPartOfAGapKeepsTheRestOpen(t *testing.T) {
	src := parsePatch(t, twoFilePatch)[0].source()
	u := unified(src.rows(map[int][]string{6: {"n6"}}))
	if want := "\na\nb\nb2\nB\ne\nn6\n\nc\nd"; u.text != want {
		t.Fatalf("text = %q, want %q", u.text, want)
	}
	if g := u.gaps[7]; g == nil || g.From != 7 || g.To != 8 || g.OldFrom != 8 {
		t.Fatalf("rest of the gap = %+v", u.gaps[7])
	}
	whole := unified(src.rows(map[int][]string{6: {"n6", "n7", "n8"}}))
	if want := "\na\nb\nb2\nB\ne\nn6\nn7\nn8\nc\nd"; whole.text != want || len(whole.gaps) != 1 {
		t.Fatalf("a gap pulled in whole must leave no separator: %q, gaps %v", whole.text, whole.gaps)
	}
}

func TestChunksNameTheirChangedLines(t *testing.T) {
	chunks := chunksOf(parsePatch(t, twoFilePatch)[0].source().rows(nil))
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	want := Chunk{
		OldStart: 4, OldCount: 2, NewStart: 4, NewCount: 1,
		Lines: []ChangedLine{{OldSide, 4, "b"}, {OldSide, 5, "b2"}, {NewSide, 4, "B"}},
	}
	if got := chunks[0]; !chunkEqual(got, want) {
		t.Fatalf("first chunk = %+v, want %+v", got, want)
	}
	if got, want := chunks[1], (Chunk{OldStart: 11, NewStart: 10, NewCount: 1, Lines: []ChangedLine{{NewSide, 10, "d"}}}); !chunkEqual(got, want) {
		t.Fatalf("second chunk = %+v, want %+v", got, want)
	}
}

func chunkEqual(a, b Chunk) bool {
	return a.OldStart == b.OldStart && a.OldCount == b.OldCount && a.NewStart == b.NewStart &&
		a.NewCount == b.NewCount && slices.Equal(a.Lines, b.Lines)
}

func TestSplitPadsWithFillersAndKeepsButtonsRight(t *testing.T) {
	left, right := split(parsePatch(t, twoFilePatch)[0].source().rows(nil))
	if want := "\na\nb\nb2\ne\n\nc\n"; left.text != want {
		t.Fatalf("left = %q, want %q", left.text, want)
	}
	if want := "\na\nB\n\ne\n\nc\nd"; right.text != want {
		t.Fatalf("right = %q, want %q", right.text, want)
	}
	if want := []string{"", "3", "4", "5", "6", "", "10", ""}; !slices.Equal(left.labels, want) {
		t.Fatalf("left labels = %q, want %q", left.labels, want)
	}
	if len(left.gaps)+len(left.reverts) != 0 || len(right.gaps) != 2 || len(right.reverts) != 2 {
		t.Fatalf("buttons: left %v %v, right %v %v", left.gaps, left.reverts, right.gaps, right.reverts)
	}
}

func TestAGapAfterAnInsertionOnlyHunkCanBeExpanded(t *testing.T) {
	// git numbers a side with no lines from the line before it: -5,0 sits
	// after old line 5, so the old file resumes at 6.
	files := parsePatch(t, "diff --git a/x b/x\n--- a/x\n+++ b/x\n"+
		"@@ -5,0 +6,2 @@\n+n1\n+n2\n"+
		"@@ -10 +12 @@\n-o\n+N\n")
	u := unified(files[0].source().rows(nil))
	var between *Gap
	for _, g := range u.gaps {
		if g.From == 8 {
			between = g
		}
	}
	if between == nil || between.To != 11 || between.OldFrom != 6 {
		t.Fatalf("gap between the hunks = %+v, want new 8..11 from old 6", between)
	}
}

// fileLines is "l1" to "ln", one a line.
func fileLines(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprint("l", i+1)
	}
	return out
}

func TestDocsWithoutCollapseShowEveryLine(t *testing.T) {
	a := strings.Join(fileLines(5), "\n")
	b := strings.Replace(a, "l3", "L3", 1)
	u := unified(docsSource(a, b, nil).rows(nil))
	if want := "l1\nl2\nl3\nL3\nl4\nl5"; u.text != want || len(u.gaps) != 0 {
		t.Fatalf("text = %q, gaps %v; want %q and no gaps", u.text, u.gaps, want)
	}
}

func TestCollapseLeavesMarginLinesAroundAChange(t *testing.T) {
	a := strings.Join(fileLines(20), "\n")
	b := strings.Replace(a, "l10\n", "L10\n", 1)
	u := unified(docsSource(a, b, &Collapse{Margin: 3, MinSize: 4}).rows(nil))
	if want := "\nl7\nl8\nl9\nl10\nL10\nl11\nl12\nl13\n"; u.text != want {
		t.Fatalf("text = %q, want %q", u.text, want)
	}
	if g := u.gaps[0]; g == nil || g.From != 1 || g.To != 6 || g.OldFrom != 1 {
		t.Fatalf("top gap = %+v, want 1..6", g)
	}
	if g := u.gaps[9]; g == nil || g.From != 14 || g.To != 20 || g.OldFrom != 14 {
		t.Fatalf("bottom gap = %+v, want 14..20", g)
	}
}

func TestCollapseKeepsARunTooShortToHide(t *testing.T) {
	a := strings.Join(fileLines(6), "\n")
	b := strings.Replace(a, "l1\n", "L1\n", 1)
	u := unified(docsSource(a, b, &Collapse{Margin: 3, MinSize: 4}).rows(nil))
	if len(u.gaps) != 0 {
		t.Fatalf("hid %v, but only 2 lines would go", u.gaps)
	}
}

func TestAnUnchangedDocumentCollapsesWhole(t *testing.T) {
	a := strings.Join(fileLines(10), "\n")
	src := docsSource(a, a, &Collapse{Margin: 3, MinSize: 4})
	u := unified(src.rows(nil))
	if g := u.gaps[0]; u.text != "" || g == nil || g.From != 1 || g.To != 10 {
		t.Fatalf("text %q, gap %+v; want one separator for 1..10", u.text, g)
	}
	if whole := unified(src.rows(map[int][]string{1: fileLines(10)})); whole.text != a {
		t.Fatalf("filled in whole it reads %q", whole.text)
	}
}
