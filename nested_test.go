package godemirror

import (
	"image"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// nestedHarness lays a View out inside a parent that scrolls vertically, as
// a panel listing several of them does, and counts what reached the parent.
type nestedHarness struct {
	*harness
	parent  int
	parentY float32
}

func newNestedHarness(t *testing.T, text string) *nestedHarness {
	t.Helper()
	h := &nestedHarness{harness: newHarness(t, text)}
	h.v.Nested = true
	h.frame()
	return h
}

func (h *nestedHarness) frame() {
	h.t.Helper()
	h.ops.Reset()
	gtx := layout.Context{
		Ops:         &h.ops,
		Now:         h.now,
		Source:      h.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(h.size),
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: h, Kinds: pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1000, Max: 1000}})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			h.parent++
			h.parentY += e.Scroll.Y
		}
	}
	area := clip.Rect{Max: h.size}.Push(gtx.Ops)
	event.Op(gtx.Ops, h)
	if _, err := h.v.Layout(gtx); err != nil {
		h.t.Fatal(err)
	}
	area.Pop()
	h.router.Frame(&h.ops)
}

func (h *nestedHarness) send(events ...event.Event) {
	h.t.Helper()
	h.router.Queue(events...)
	h.frame()
}

// longLines is a document wider than the view and exactly as tall as it, the
// shape a diff card takes inside a panel: nothing to scroll down, much to
// scroll sideways.
func longLines(h *harness) string {
	rows := h.size.Y / h.v.r.CellH()
	return strings.TrimSuffix(strings.Repeat(strings.Repeat("x", 300)+"\n", rows), "\n")
}

func TestNestedViewLeavesThePlainWheelToItsParent(t *testing.T) {
	h := newNestedHarness(t, "")
	h.v.SetContent(Content{Text: longLines(h.harness)})
	h.frame()
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 100), Scroll: f32.Pt(0, 50)})
	if h.parent != 1 || h.parentY != 50 {
		t.Fatalf("parent got %d wheel events (%v), want the one of 50", h.parent, h.parentY)
	}
	if h.v.scroll != (image.Point{}) {
		t.Errorf("view scrolled to %v, want it left alone", h.v.scroll)
	}
}

func TestNestedViewStillScrollsSidewaysWithShift(t *testing.T) {
	h := newNestedHarness(t, "")
	h.v.SetContent(Content{Text: longLines(h.harness)})
	h.frame()
	h.send(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(200, 100), Modifiers: key.ModShift})
	// The router hands the wheel to the ranges of the frame before it, so
	// the view claims it from the frame after it saw Shift.
	h.frame()
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 100), Scroll: f32.Pt(0, 50), Modifiers: key.ModShift})
	if h.v.scroll.X != 50 || h.parent != 0 {
		t.Fatalf("scroll = %v and parent got %d, want (50,0) and none", h.v.scroll, h.parent)
	}
}

func TestNestedViewTakesTheWheelItCanScrollDown(t *testing.T) {
	h := newNestedHarness(t, strings.Repeat("line\n", 100))
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 100), Scroll: f32.Pt(0, 50)})
	if h.v.scroll.Y != 50 || h.parent != 0 {
		t.Fatalf("scroll = %v and parent got %d, want (0,50) and none", h.v.scroll, h.parent)
	}
}
