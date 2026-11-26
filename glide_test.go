package godemirror

import (
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

// settle lays the view out until a scroll the program asked for since the
// last frame has glided all the way: one frame starts the glide, a later
// one past its duration lands it.
func (h *harness) settle() {
	h.t.Helper()
	h.frame()
	h.advance(glideDuration)
}

func TestPageDownGlidesToItsTargetAndThenAsksForNoFrames(t *testing.T) {
	// Position 50 starts line 10, on screen, so PageDown scrolls a page.
	h, _ := focusedEditor(t, EditorStateConfig{Doc: manyLines(200)}, SingleSelection(50, 50))
	calls := 0
	h.ed.OnScroll = func() { calls++ }
	h.press(key.NamePageDown, 0)
	target := h.ed.ScrollTarget()
	if target <= 3*h.v.r.CellH() || h.ed.ScrollTop() != 0 || !h.redrawRequested() {
		t.Fatalf("PageDown at %d heading for %d, redraw %v: want a glide from 0 under way", h.ed.ScrollTop(), target, h.redrawRequested())
	}
	h.advance(glideDuration / 2)
	if mid := h.ed.ScrollTop(); mid <= target/2 || mid >= target || !h.redrawRequested() {
		t.Fatalf("halfway the view is at %d of %d: want past half, eased out, and more frames", mid, target)
	}
	h.advance(glideDuration / 2)
	if h.ed.ScrollTop() != target || h.redrawRequested() {
		t.Fatalf("at the end the view is at %d of %d, redraw %v", h.ed.ScrollTop(), target, h.redrawRequested())
	}
	if calls != 0 {
		t.Fatalf("the glide called OnScroll %d times, want none", calls)
	}
}

func TestAnIdleViewAsksForNoFrameAfterAGlide(t *testing.T) {
	h := newHarness(t, manyLines(200))
	waitForHighlight(h)
	h.v.SetScrollTop(100 * h.v.r.CellH())
	h.settle()
	if h.v.ScrollTop() != 100*h.v.r.CellH() {
		t.Fatalf("SetScrollTop landed at %d", h.v.ScrollTop())
	}
	h.advance(time.Second)
	if _, ok := h.router.WakeupTime(); ok {
		t.Fatal("an idle view asked for a frame")
	}
}

func TestAnIdleEditorAsksForNoFrameAfterAGlide(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: manyLines(200)})
	waitForHighlight(h)
	h.ed.SetScrollTop(100 * h.v.r.CellH())
	h.settle()
	h.advance(time.Second)
	if at, ok := h.router.WakeupTime(); ok {
		t.Fatalf("an idle editor asked for a frame at %v", at)
	}
}

func TestAShortScrollIntoViewSnaps(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: manyLines(200)}, SingleSelection(0, 0))
	rows := h.size.Y / h.v.r.CellH()
	for range rows {
		h.press(key.NameDownArrow, 0)
	}
	if h.ed.ScrollTop() == 0 || h.ed.ScrollTop() != h.ed.ScrollTarget() || h.redrawRequested() {
		t.Fatalf("an arrow past the bottom: at %d heading for %d: want a snap", h.ed.ScrollTop(), h.ed.ScrollTarget())
	}
}

func TestTheWheelStopsAGlideWhereTheViewIs(t *testing.T) {
	h := newHarness(t, manyLines(200))
	calls := 0
	h.v.OnScroll = func() { calls++ }
	h.v.SetScrollTop(100 * h.v.r.CellH())
	h.frame()
	h.advance(glideDuration / 2)
	mid := h.v.ScrollTop()
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(100, 100), Scroll: f32.Pt(0, 10)})
	if h.v.ScrollTop() != mid+10 || h.v.ScrollTarget() != mid+10 || calls != 1 {
		t.Fatalf("a wheel of 10 at %d: at %d heading for %d, %d calls", mid, h.v.ScrollTop(), h.v.ScrollTarget(), calls)
	}
	h.advance(glideDuration)
	if h.v.ScrollTop() != mid+10 || h.redrawRequested() {
		t.Fatalf("after the wheel the view went on to %d", h.v.ScrollTop())
	}
}

func TestSnapScrollTopAndReduceMotionLandAtOnce(t *testing.T) {
	h := newHarness(t, manyLines(200))
	far := 100 * h.v.r.CellH()
	h.v.SnapScrollTop(far)
	h.frame()
	if h.v.ScrollTop() != far || h.redrawRequested() {
		t.Fatalf("SnapScrollTop(%d): at %d", far, h.v.ScrollTop())
	}
	h.v.ReduceMotion = true
	h.v.SetScrollTop(0)
	h.frame()
	if h.v.ScrollTop() != 0 || h.redrawRequested() {
		t.Fatalf("SetScrollTop(0) with ReduceMotion: at %d", h.v.ScrollTop())
	}
	h.v.ScrollToLine(150, AlignTop)
	h.frame()
	if h.v.ScrollTop() != h.v.ScrollTarget() || h.v.ScrollTop() == 0 {
		t.Fatalf("ScrollToLine with ReduceMotion: at %d heading for %d", h.v.ScrollTop(), h.v.ScrollTarget())
	}
}

func TestAnEditorWithReduceMotionPagesAtOnce(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: manyLines(200)}, SingleSelection(0, 0))
	h.ed.ReduceMotion = true
	h.press(key.NamePageDown, 0)
	if h.ed.ScrollTop() == 0 || h.ed.ScrollTop() != h.ed.ScrollTarget() || h.redrawRequested() {
		t.Fatalf("PageDown with ReduceMotion: at %d heading for %d", h.ed.ScrollTop(), h.ed.ScrollTarget())
	}
}
