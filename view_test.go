package godemirror

import (
	"image"
	"maps"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// harness drives a View, or a Split when split is set, through Gio's own
// input router, frame by frame.
type harness struct {
	t      *testing.T
	v      *View
	split  *Split
	diff   *DiffView
	size   image.Point
	ops    op.Ops
	router input.Router
	now    time.Time
}

func newHarness(t *testing.T, text string) *harness {
	t.Helper()
	v, err := New("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	v.SetContent(Content{Text: text})
	h := &harness{t: t, v: v, size: image.Pt(400, 200), now: time.Unix(1, 0)}
	h.frame()
	return h
}

func (h *harness) frame() {
	h.t.Helper()
	h.ops.Reset()
	gtx := layout.Context{
		Ops:         &h.ops,
		Now:         h.now,
		Source:      h.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(h.size),
	}
	var err error
	switch {
	case h.diff != nil:
		_, err = h.diff.Layout(gtx)
	case h.split != nil:
		_, err = h.split.Layout(gtx)
	default:
		_, err = h.v.Layout(gtx)
	}
	if err != nil {
		h.t.Fatal(err)
	}
	h.router.Frame(&h.ops)
}

// send queues events and lays the view out again so it takes them.
func (h *harness) send(events ...event.Event) {
	h.t.Helper()
	h.router.Queue(events...)
	h.frame()
}

// at is the middle of the left edge of cell col on line, with no scrolling.
func (h *harness) at(line, col int) f32.Point {
	cw, ch := h.v.r.CellW(), h.v.r.CellH()
	g := h.v.geometry(h.size, cw)
	return f32.Pt(float32(g.textX+col*cw+1), float32(line*ch+ch/2))
}

func (h *harness) pointer(kind pointer.Kind, p f32.Point, mods key.Modifiers) {
	h.t.Helper()
	h.send(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: p, Buttons: pointer.ButtonPrimary, Modifiers: mods})
}

func (h *harness) click(p f32.Point, mods key.Modifiers) {
	h.t.Helper()
	h.pointer(pointer.Press, p, mods)
	h.pointer(pointer.Release, p, mods)
}

func TestShiftWheelScrollsSideways(t *testing.T) {
	h := newHarness(t, strings.Repeat(strings.Repeat("x", 300)+"\n", 3))
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 100), Scroll: f32.Pt(0, 50), Modifiers: key.ModShift})
	if h.v.scroll.X != 50 || h.v.scroll.Y != 0 {
		t.Fatalf("scroll = %v, want (50,0)", h.v.scroll)
	}
}

func TestWheelWithoutShiftScrollsDown(t *testing.T) {
	h := newHarness(t, strings.Repeat(strings.Repeat("x", 300)+"\n", 100))
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 100), Scroll: f32.Pt(0, 50)})
	if h.v.scroll.X != 0 || h.v.scroll.Y != 50 {
		t.Fatalf("scroll = %v, want (0,50)", h.v.scroll)
	}
}

func TestTurningWrapOffShowsWholeLinesAgain(t *testing.T) {
	h := newHarness(t, "")
	cw := h.v.r.CellW()
	cols := h.v.geometry(h.size, cw).cols
	// A line a little wider than the view: wrapped it breaks in two, and
	// unwrapped it scrolls a few cells, never leaving the first page.
	line := strings.Repeat("word ", (cols+10)/5) + "end"
	h.v.SetContent(Content{Text: line})
	h.v.Wrap = true
	h.frame()
	h.v.Wrap = false
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 10), Scroll: f32.Pt(1<<16, 0)})
	img := h.drawnRow(0)
	if want := len(line) * cw; img.size.X != want {
		t.Fatalf("row image is %dpx wide, want %dpx: the line's end is cut off", img.size.X, want)
	}
}

func TestScrolledToTheEndTheLongestLineKeepsItsMargin(t *testing.T) {
	h := newHarness(t, strings.Repeat("x", 300))
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 10), Scroll: f32.Pt(1<<16, 0)})
	cw := h.v.r.CellW()
	g := h.v.geometry(h.size, cw)
	lineEnd := g.textX + 300*cw - h.v.scroll.X
	if margin := h.size.X - lineEnd; margin != int(scrollEndMargin) {
		t.Fatalf("the line ends %dpx from the edge, want %dpx", margin, int(scrollEndMargin))
	}
}

func TestGrowingTheViewShowsMoreOfEachLine(t *testing.T) {
	line := strings.Repeat("x", 120)
	h := newHarness(t, line)
	h.size = image.Pt(200, 200)
	h.frame()
	h.size = image.Pt(1200, 200)
	h.frame()
	img := h.drawnRow(0)
	if want := len(line) * h.v.r.CellW(); img.size.X != want {
		t.Fatalf("row image is %dpx wide, want %dpx: the line is still cut where the small view ended", img.size.X, want)
	}
}

// drawnRow is the image the last frame drew for line's first row.
func (h *harness) drawnRow(line int) *rowImage {
	h.t.Helper()
	for k, img := range h.v.rows {
		if k.line == line && k.segment == 0 && img.used == h.v.frameNo {
			return img
		}
	}
	h.t.Fatalf("line %d was not drawn", line)
	return nil
}

func TestLineTextIsTheLineAsWritten(t *testing.T) {
	h := newHarness(t, "a\r\n\tb\n")
	if got := h.v.LineCount(); got != 2 {
		t.Fatalf("LineCount() = %d, want 2", got)
	}
	if got := h.v.LineText(1); got != "\tb" {
		t.Fatalf("LineText(1) = %q, want %q", got, "\tb")
	}
}

func TestChangingTheThemeRecolorsTheRowsOnScreen(t *testing.T) {
	h := newHarness(t, "package main")
	row, label := h.drawnRow(0), h.v.gutter[0]
	h.v.Theme = LightTheme
	h.frame()
	if h.drawnRow(0) == row {
		t.Fatal("line 0 is still the image composed in the old theme's colors")
	}
	if h.v.gutter[0] == label {
		t.Fatal("line 0's label is still the image composed in the old theme's colors")
	}
}

// drawnRows is every row image the last frame drew, by what it holds.
func (h *harness) drawnRows() map[rowKey]image.Point {
	drawn := map[rowKey]image.Point{}
	for k, img := range h.v.rows {
		if img.used == h.v.frameNo {
			drawn[k] = img.size
		}
	}
	return drawn
}

func TestResizedRowsShowWhatAFreshViewShows(t *testing.T) {
	text := strings.Repeat("short\n"+strings.Repeat("word ", 40)+"\n"+strings.Repeat("x", 150)+"\n", 5)
	for _, wrap := range []bool{false, true} {
		dragged := newHarness(t, text)
		dragged.v.Wrap = wrap
		for _, width := range []int{400, 403, 520, 260, 1300, 410} {
			dragged.size = image.Pt(width, 200)
			dragged.frame()
			fresh := newHarness(t, text)
			fresh.v.Wrap = wrap
			fresh.size = dragged.size
			fresh.frame()
			if got, want := dragged.drawnRows(), fresh.drawnRows(); !maps.Equal(got, want) {
				t.Fatalf("wrap=%v, resized to %dpx: drew rows %v, a fresh view draws %v", wrap, width, got, want)
			}
		}
	}
}

func TestNarrowingTheViewKeepsItsRowImages(t *testing.T) {
	h := newHarness(t, strings.Repeat(strings.Repeat("x", 300)+"\nshort\n", 5))
	before := maps.Clone(h.v.rows)
	h.size.X -= 2 * h.v.r.CellW()
	h.frame()
	for k, img := range h.v.rows {
		if before[k] != img {
			t.Fatalf("row %v was composed again after the view narrowed by two cells", k)
		}
	}
}
