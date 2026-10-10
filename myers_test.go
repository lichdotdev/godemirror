package godemirror

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// lcsLength is the longest common subsequence of a and b, by dynamic
// programming: the number of lines a minimal edit script keeps.
func lcsLength(a, b []string) int {
	row := make([]int, len(b)+1)
	for i := range a {
		prev := 0
		for j := range b {
			cur := row[j+1]
			if a[i] == b[j] {
				row[j+1] = prev + 1
			} else {
				row[j+1] = max(row[j+1], row[j])
			}
			prev = cur
		}
	}
	return row[len(b)]
}

// replay rebuilds both sides from an edit script.
func replay(edits []edit) (a, b []string) {
	for _, e := range edits {
		if e.op != opInsert {
			a = append(a, e.text)
		}
		if e.op != opDelete {
			b = append(b, e.text)
		}
	}
	return a, b
}

func randomLines(r *rand.Rand, n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = string(rune('a' + r.IntN(4)))
	}
	return lines
}

func TestDiffLinesIsAMinimalEditScript(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		a, b := randomLines(r, r.IntN(40)), randomLines(r, r.IntN(40))
		edits := diffLines(a, b, maxDiffEdits)
		gotA, gotB := replay(edits)
		if !slices.Equal(gotA, a) || !slices.Equal(gotB, b) {
			t.Fatalf("diff of %q and %q replays as %q and %q", a, b, gotA, gotB)
		}
		kept := 0
		for _, e := range edits {
			if e.op == opEqual {
				kept++
			}
		}
		if want := lcsLength(a, b); kept != want {
			t.Fatalf("diff of %q and %q keeps %d lines, want %d", a, b, kept, want)
		}
	}
}

func TestDiffLinesPutsDeletionsBeforeInsertions(t *testing.T) {
	edits := diffLines([]string{"a", "b", "c"}, []string{"a", "B", "c"}, maxDiffEdits)
	want := []edit{{opEqual, "a"}, {opDelete, "b"}, {opInsert, "B"}, {opEqual, "c"}}
	if !slices.Equal(edits, want) {
		t.Fatalf("got %v, want %v", edits, want)
	}
}

func TestDiffLinesPastTheCapReplacesTheRestWhole(t *testing.T) {
	a := []string{"same", "1", "2", "3", "4", "end"}
	b := []string{"same", "5", "6", "7", "8", "end"}
	edits := diffLines(a, b, 2)
	gotA, gotB := replay(edits)
	if !slices.Equal(gotA, a) || !slices.Equal(gotB, b) {
		t.Fatalf("replays as %q and %q", gotA, gotB)
	}
	want := []edit{
		{opEqual, "same"},
		{opDelete, "1"}, {opDelete, "2"}, {opDelete, "3"}, {opDelete, "4"},
		{opInsert, "5"}, {opInsert, "6"}, {opInsert, "7"}, {opInsert, "8"},
		{opEqual, "end"},
	}
	if !slices.Equal(edits, want) {
		t.Fatalf("got %v, want %v", edits, want)
	}
}
