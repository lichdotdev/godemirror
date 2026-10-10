package godemirror

import (
	"slices"
	"testing"
)

func TestChangedWordsAreMarkedOnBothLines(t *testing.T) {
	old, new := changedSpans("call(a, b)", "call(a, cc)")
	if want := []Span{{From: 8, To: 9}}; !slices.Equal(old, want) {
		t.Fatalf("old spans = %v, want %v", old, want)
	}
	if want := []Span{{From: 8, To: 10}}; !slices.Equal(new, want) {
		t.Fatalf("new spans = %v, want %v", new, want)
	}
}

func TestSpansCountRunesNotBytes(t *testing.T) {
	old, new := changedSpans("é x", "é y")
	if want := []Span{{From: 2, To: 3}}; !slices.Equal(old, want) || !slices.Equal(new, want) {
		t.Fatalf("spans = %v | %v, want %v on both", old, new, want)
	}
}

func TestLinesWithNothingInCommonGetNoSpans(t *testing.T) {
	if old, new := changedSpans("alpha beta", "gamma delta"); old != nil || new != nil {
		t.Fatalf("spans = %v | %v, want none: the whole line changed", old, new)
	}
}

func TestDiffViewHighlightsChangesOnlyWhenAsked(t *testing.T) {
	for _, on := range []bool{false, true} {
		h, d := newDiffHarness(t, DiffOptions{HighlightChanges: on})
		d.SetDocs("main.go", "a\ncall(a, b)\nc", "a\ncall(a, cc)\nc")
		h.frame()
		deleted, added := h.v.decoration(1), h.v.decoration(2)
		if !on {
			if deleted.Spans != nil || added.Spans != nil {
				t.Fatal("spans without HighlightChanges")
			}
			continue
		}
		if want := []Span{{8, 9, DarkTheme.Diff.Deleted.Changed}}; !slices.Equal(deleted.Spans, want) {
			t.Fatalf("deleted row spans = %v, want %v", deleted.Spans, want)
		}
		if want := []Span{{8, 10, DarkTheme.Diff.Added.Changed}}; !slices.Equal(added.Spans, want) {
			t.Fatalf("added row spans = %v, want %v", added.Spans, want)
		}
	}
}

func TestASpanAfterATabCoversItsCells(t *testing.T) {
	h := newHarness(t, "\tab")
	if from, to := h.v.spanCells(0, Span{From: 1, To: 2}); from != 4 || to != 5 {
		t.Fatalf("span over 'a' covers cells %d..%d, want 4..5", from, to)
	}
}
