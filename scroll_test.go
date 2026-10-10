package godemirror

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
)

// numbered is n lines, "line 0" to "line n-1".
func numbered(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprint("line ", i)
	}
	return strings.Join(lines, "\n")
}

// lineAtY is the line a triple click at y in the view selects.
func (h *harness) lineAtY(y float32) int {
	h.t.Helper()
	p := f32.Pt(h.at(0, 1).X, y)
	h.now = h.now.Add(time.Second)
	for range 3 {
		h.click(p, 0)
	}
	from, _, ok := h.v.SelectedLines()
	if !ok {
		h.t.Fatalf("nothing selected at y=%v", y)
	}
	return from
}

func TestScrollToLineTopPutsTheLineFirst(t *testing.T) {
	h := newHarness(t, numbered(100))
	h.v.ScrollToLine(50, AlignTop)
	h.frame()
	if got := h.lineAtY(1); got != 50 {
		t.Fatalf("top line is %d, want 50", got)
	}
}

func TestScrollToLineCenterPutsTheLineMidway(t *testing.T) {
	h := newHarness(t, numbered(100))
	h.v.ScrollToLine(50, AlignCenter)
	h.frame()
	if got := h.lineAtY(float32(h.size.Y) / 2); got != 50 {
		t.Fatalf("middle line is %d, want 50", got)
	}
}

func TestScrollToLineNearestLeavesAVisibleLineAlone(t *testing.T) {
	h := newHarness(t, numbered(100))
	h.v.ScrollToLine(3, AlignNearest)
	h.frame()
	if got := h.lineAtY(1); got != 0 {
		t.Fatalf("top line is %d, want 0", got)
	}
}

func TestScrollToLineNearestBringsALineBelowToTheBottom(t *testing.T) {
	h := newHarness(t, numbered(100))
	h.v.ScrollToLine(50, AlignNearest)
	h.frame()
	ch := float32(h.v.r.CellH())
	if got := h.lineAtY(float32(h.size.Y) - ch/2); got != 50 {
		t.Fatalf("bottom line is %d, want 50", got)
	}
}

func TestScrollToLineStopsAtTheEnd(t *testing.T) {
	h := newHarness(t, numbered(100))
	h.v.ScrollToLine(99, AlignTop)
	h.frame()
	ch := float32(h.v.r.CellH())
	if got := h.lineAtY(float32(h.size.Y) - ch/2); got != 99 {
		t.Fatalf("bottom line is %d, want 99", got)
	}
}

func TestScrollToLineOnOneSideOfASplitScrollsBoth(t *testing.T) {
	split, err := NewSplit("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	split.Left.SetContent(Content{Text: numbered(100)})
	split.Right.SetContent(Content{Text: numbered(100)})
	h := &harness{t: t, v: split.Right, split: split, size: image.Pt(800, 200), now: time.Unix(1, 0)}
	h.frame()
	split.Left.ScrollToLine(50, AlignTop)
	h.frame()
	p := h.at(0, 1)
	p.X += float32(800 / 2)
	for range 3 {
		h.click(p, 0)
	}
	if from, _, _ := split.Right.SelectedLines(); from != 50 {
		t.Fatalf("right side's top line is %d, want 50", from)
	}
}
