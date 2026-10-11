package godemirror

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

const autoScrollTick = 100 * time.Millisecond

// manyLines is n short lines, long enough to scroll down but never sideways.
func manyLines(n int) string { return strings.Repeat("line\n", n) }

// advance moves the clock and lays the view out again with no event, so
// redrawRequested then speaks for that frame alone.
func (h *harness) advance(d time.Duration) {
	h.t.Helper()
	h.router.WakeupTime()
	h.now = h.now.Add(d)
	h.frame()
}

// redrawRequested reports whether the last frame asked for another one at
// once, as auto-scroll does, rather than at a later time as highlighting does.
func (h *harness) redrawRequested() bool {
	at, ok := h.router.WakeupTime()
	return ok && at.IsZero()
}

// below is the point dist px under the bottom edge of the view, over the
// code of the first column.
func (h *harness) below(dist float32) f32.Point {
	p := h.at(0, 1)
	p.Y = float32(h.size.Y) + dist
	return p
}

// beyondRight is the point dist px past the right edge, on the first row.
func (h *harness) beyondRight(dist float32) f32.Point {
	p := h.at(0, 1)
	p.X = float32(h.size.X) + dist
	return p
}

func TestDraggingBelowTheViewScrollsAndExtendsTheSelectionWithoutPointerEvents(t *testing.T) {
	h := newHarness(t, manyLines(100))
	h.pointer(pointer.Press, h.at(1, 0), 0)
	h.pointer(pointer.Move, h.below(30), 0)
	_, to, _ := h.v.SelectedLines()
	h.advance(autoScrollTick)
	first := h.v.scroll.Y
	if first <= 0 {
		t.Fatalf("scroll.Y = %d after a frame with the pointer held below the view, want it to have moved down", first)
	}
	if _, now, _ := h.v.SelectedLines(); now <= to {
		t.Fatalf("selection ends on line %d, was %d: it did not extend as the view scrolled", now, to)
	}
	h.advance(autoScrollTick)
	if h.v.scroll.Y <= first {
		t.Fatalf("scroll.Y = %d, was %d: it stopped while the pointer stayed below", h.v.scroll.Y, first)
	}
	if !h.redrawRequested() {
		t.Fatal("no next frame was requested while auto-scrolling")
	}
}

func TestAutoScrollTakesTimeFromTheFrameNotTheFrameCount(t *testing.T) {
	h := newHarness(t, manyLines(100))
	h.pointer(pointer.Press, h.at(1, 0), 0)
	h.pointer(pointer.Move, h.below(30), 0)
	h.frame()
	h.frame()
	if h.v.scroll.Y != 0 {
		t.Fatalf("scroll.Y = %d after frames at the same instant, want 0", h.v.scroll.Y)
	}
}

// scrolledBelow is how far the view scrolled in one tick with the pointer dist
// px below it.
func scrolledBelow(t *testing.T, dist float32) int {
	t.Helper()
	h := newHarness(t, manyLines(1000))
	h.pointer(pointer.Press, h.at(1, 0), 0)
	h.pointer(pointer.Move, h.below(dist), 0)
	h.advance(autoScrollTick)
	return h.v.scroll.Y
}

func TestAutoScrollIsFasterTheFartherThePointerIsBeyondTheEdge(t *testing.T) {
	near, far := scrolledBelow(t, 5), scrolledBelow(t, 60)
	if near <= 0 || far <= near {
		t.Fatalf("scrolled %d px at 5px beyond and %d at 60px: want some, and more with distance", near, far)
	}
}

func TestAutoScrollSpeedIsCapped(t *testing.T) {
	far, farther := scrolledBelow(t, 5000), scrolledBelow(t, 50000)
	if far != farther {
		t.Fatalf("scrolled %d px at 5000px beyond and %d at 50000px: want the same, capped", far, farther)
	}
}

func TestAutoScrollStopsAtTheEndOfTheDocument(t *testing.T) {
	h := newHarness(t, manyLines(30))
	h.pointer(pointer.Press, h.at(1, 0), 0)
	h.pointer(pointer.Move, h.below(80), 0)
	for range 100 {
		h.advance(autoScrollTick)
	}
	end := h.v.tops[len(h.v.tops)-1] - h.size.Y
	if h.v.scroll.Y != end {
		t.Fatalf("scroll.Y = %d, want the end of the document at %d", h.v.scroll.Y, end)
	}
	last := h.v.LineCount() - 1
	if _, to, _ := h.v.SelectedLines(); to != last {
		t.Fatalf("selection ends on line %d, want the last line %d", to, last)
	}
	h.advance(autoScrollTick)
	if h.redrawRequested() {
		t.Fatal("frames are still requested at the end of the document")
	}
}

func TestAutoScrollGoesUpWhenDraggedAboveTheView(t *testing.T) {
	h := newHarness(t, manyLines(100))
	h.v.ScrollToLine(60, AlignTop)
	h.frame()
	start := h.v.scroll.Y
	h.pointer(pointer.Press, h.at(1, 0), 0)
	p := h.at(0, 1)
	p.Y = -40
	h.pointer(pointer.Move, p, 0)
	h.advance(autoScrollTick)
	if h.v.scroll.Y >= start {
		t.Fatalf("scroll.Y = %d, was %d: want it to have moved up", h.v.scroll.Y, start)
	}
}

func TestAutoScrollGoesSidewaysPastTheRightEdgeWhenUnwrapped(t *testing.T) {
	h := newHarness(t, strings.Repeat("x", 300))
	h.pointer(pointer.Press, h.at(0, 1), 0)
	h.pointer(pointer.Move, h.beyondRight(30), 0)
	h.advance(autoScrollTick)
	if h.v.scroll.X <= 0 || h.v.scroll.Y != 0 {
		t.Fatalf("scroll = %v, want it moved right only", h.v.scroll)
	}
	if from, to, ok := h.v.Selection(); !ok || to.Col <= from.Col+1 {
		t.Fatalf("selection = %v..%v %v, want it extended along the line", from, to, ok)
	}
}

func TestAutoScrollGoesLeftOverTheGutterWhenUnwrapped(t *testing.T) {
	h := newHarness(t, strings.Repeat("x", 300))
	h.v.scroll.X = 400
	h.frame()
	h.pointer(pointer.Press, h.at(0, 20), 0)
	h.pointer(pointer.Move, f32.Pt(1, h.at(0, 0).Y), 0)
	h.advance(autoScrollTick)
	if h.v.scroll.X >= 400 {
		t.Fatalf("scroll.X = %d, was 400: want it to have moved left", h.v.scroll.X)
	}
}

func TestAutoScrollDoesNotGoSidewaysWhenWrapped(t *testing.T) {
	h := newHarness(t, strings.Repeat("x", 300))
	h.v.Wrap = true
	h.frame()
	h.pointer(pointer.Press, h.at(0, 1), 0)
	h.pointer(pointer.Move, h.beyondRight(30), 0)
	h.advance(autoScrollTick)
	if h.v.scroll != (image.Point{}) {
		t.Fatalf("scroll = %v, want none", h.v.scroll)
	}
	if h.redrawRequested() {
		t.Fatal("frames were requested with nothing to scroll")
	}
}

func TestReleasingStopsAutoScroll(t *testing.T) {
	h := newHarness(t, manyLines(100))
	h.pointer(pointer.Press, h.at(1, 0), 0)
	h.pointer(pointer.Move, h.below(30), 0)
	h.advance(autoScrollTick)
	h.pointer(pointer.Release, h.below(30), 0)
	stopped := h.v.scroll.Y
	h.advance(autoScrollTick)
	h.advance(autoScrollTick)
	if h.v.scroll.Y != stopped {
		t.Fatalf("scroll.Y = %d, was %d at release: it went on after the button came up", h.v.scroll.Y, stopped)
	}
	if h.redrawRequested() {
		t.Fatal("frames are still requested after release")
	}
}

func TestPointerBackInsideStopsAutoScroll(t *testing.T) {
	h := newHarness(t, manyLines(100))
	h.pointer(pointer.Press, h.at(1, 0), 0)
	h.pointer(pointer.Move, h.below(30), 0)
	h.advance(autoScrollTick)
	h.pointer(pointer.Move, h.at(5, 0), 0)
	stopped := h.v.scroll.Y
	h.advance(autoScrollTick)
	if h.v.scroll.Y != stopped {
		t.Fatalf("scroll.Y = %d, was %d when the pointer came back: it went on", h.v.scroll.Y, stopped)
	}
	if h.redrawRequested() {
		t.Fatal("frames are still requested with the pointer inside")
	}
}

func TestAnIdleViewRequestsNoFrames(t *testing.T) {
	h := newHarness(t, manyLines(100))
	h.pointer(pointer.Press, h.at(1, 0), 0)
	h.pointer(pointer.Move, h.at(5, 0), 0)
	h.advance(autoScrollTick)
	if h.redrawRequested() {
		t.Fatal("a drag inside the view requests frames")
	}
}

func TestSelectDoesNotScroll(t *testing.T) {
	h := newHarness(t, manyLines(100))
	h.v.Select(Pos{0, 0}, Pos{90, 0})
	h.advance(autoScrollTick)
	if h.v.scroll != (image.Point{}) {
		t.Fatalf("scroll = %v after Select, want none", h.v.scroll)
	}
}

func TestSplitScrollsBothSidesWhenOneIsDraggedBelow(t *testing.T) {
	split, err := NewSplit("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	split.Left.SetContent(Content{Text: manyLines(100)})
	split.Right.SetContent(Content{Text: manyLines(100)})
	h := &harness{t: t, v: split.Left, split: split, size: image.Pt(800, 200), now: time.Unix(1, 0)}
	h.frame()
	h.pointer(pointer.Press, h.at(1, 0), 0)
	p := h.at(0, 1)
	p.Y = float32(h.size.Y) + 30
	h.pointer(pointer.Move, p, 0)
	h.advance(autoScrollTick)
	h.advance(autoScrollTick)
	if split.Left.scroll.Y <= 0 || split.Right.scroll.Y != split.Left.scroll.Y {
		t.Fatalf("scroll.Y = %d left, %d right: want both scrolled and level", split.Left.scroll.Y, split.Right.scroll.Y)
	}
	if _, to, _ := split.Left.SelectedLines(); to <= 1 {
		t.Fatalf("left selection ends on line %d, want it extended", to)
	}
}
