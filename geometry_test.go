package godemirror

import (
	"image"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
)

func TestLineBlockCoversALinesRowsAndNotTheBlocksUnderIt(t *testing.T) {
	h := newHarness(t, "")
	ch := h.v.r.CellH()
	long := strings.Repeat("word ", 200)
	h.v.SetContent(Content{Text: "one\n" + long + "\nthree", Blocks: []BlockWidget{{After: 0, Widget: func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(10, 30)}
	}}}})
	h.v.Wrap = true
	h.frame()
	if top, bottom := h.v.LineBlock(0); top != 0 || bottom != ch {
		t.Fatalf("line 1 at %d..%d, want 0..%d", top, bottom, ch)
	}
	top, bottom := h.v.LineBlock(1)
	if top != ch+30 || (bottom-top)/ch != len(h.v.wraps[1]) {
		t.Fatalf("line 2 at %d..%d, want from %d over its %d rows", top, bottom, ch+30, len(h.v.wraps[1]))
	}
	if got := h.v.LineAtHeight(ch + 10); got != 0 {
		t.Fatalf("over the block is line %d, want the line it sits under", got)
	}
	if h.v.LineAtHeight(-5) != 0 || h.v.LineAtHeight(1<<20) != 2 || h.v.LineAtHeight(bottom) != 2 {
		t.Fatal("heights past the document's ends or at a line's top land elsewhere")
	}
	if want := bottom + ch; h.v.ContentHeight() != want {
		t.Fatalf("content height %d, want %d", h.v.ContentHeight(), want)
	}
}

func TestOnScrollHearsOfEveryScrollOnce(t *testing.T) {
	h := newHarness(t, manyLines(200))
	calls := 0
	h.v.OnScroll = func() { calls++ }
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(100, 100), Scroll: f32.Pt(0, 40)})
	if calls != 1 || h.v.ScrollTop() != 40 {
		t.Fatalf("the wheel: %d calls, scrolled to %d", calls, h.v.ScrollTop())
	}
	h.frame()
	if calls != 1 {
		t.Fatal("a frame that did not scroll called OnScroll")
	}
	h.v.SetScrollTop(1 << 20)
	h.frame()
	if calls != 2 || h.v.ScrollTop() != h.v.ContentHeight()-h.size.Y {
		t.Fatalf("SetScrollTop past the end: %d calls, scrolled to %d", calls, h.v.ScrollTop())
	}
}

func TestAnEditorsGeometryIsTheLastFramesAndItsScrollIsHeard(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: manyLines(200)}, SingleSelection(0, 0))
	ch := h.v.r.CellH()
	scrolled := 0
	h.ed.OnScroll = func() { scrolled++ }
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(0, "new\n")}})
	// Position 4 ends the first line before the edit, and starts the
	// second after it.
	if top, _ := h.ed.LineBlockAt(4); top != 0 {
		t.Fatalf("before a frame, position 4 is on the row at %d, want the last frame's first line, 0", top)
	}
	h.frame()
	if top, _ := h.ed.LineBlockAt(4); top != ch {
		t.Fatalf("after it, position 4 is on the row at %d, want %d", top, ch)
	}
	if l := h.ed.LineBlockAtHeight(2*ch + 1); l.Number != 3 {
		t.Fatalf("height %d is line %d, want 3", 2*ch+1, l.Number)
	}
	h.press(key.NameEnd, key.ModShortcut)
	if scrolled != 1 || h.ed.ScrollTop() == 0 {
		t.Fatalf("Ctrl+End scrolled to %d with %d OnScroll calls", h.ed.ScrollTop(), scrolled)
	}
	h.ed.SetScrollTop(0)
	h.frame()
	if scrolled != 2 || h.ed.ScrollTop() != 0 {
		t.Fatalf("SetScrollTop(0): at %d, %d calls", h.ed.ScrollTop(), scrolled)
	}
}

func TestGeometryBeforeANewContentIsLaidOutIsEstimated(t *testing.T) {
	h := newHarness(t, "")
	ch := h.v.r.CellH()
	h.v.SetContent(Content{Text: "a\nb\nc\nd"})
	if top, bottom := h.v.LineBlock(2); top != 2*ch || bottom != 3*ch {
		t.Fatalf("line 3 before a Layout at %d..%d, want a row a line, %d..%d", top, bottom, 2*ch, 3*ch)
	}
	if got := h.v.LineAtHeight(3*ch + 1); got != 3 {
		t.Fatalf("height %d before a Layout is line %d, want 3", 3*ch+1, got)
	}
	if got := h.v.ContentHeight(); got != 4*ch {
		t.Fatalf("content height before a Layout %d, want %d", got, 4*ch)
	}
	h.frame()
	if top, _ := h.v.LineBlock(2); top != 2*ch {
		t.Fatalf("line 3 after the Layout at %d, want %d", top, 2*ch)
	}
}

func TestAnEditorsGeometryBeforeANewStateIsLaidOutIsEstimated(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: ""})
	ch := h.v.r.CellH()
	if err := h.ed.SetState(EditorStateConfig{Doc: "a\nb\nc"}); err != nil {
		t.Fatal(err)
	}
	if top, _ := h.ed.LineBlockAt(4); top != 2*ch {
		t.Fatalf("position 4 before a Layout on the row at %d, want line 3's, %d", top, 2*ch)
	}
	if l := h.ed.LineBlockAtHeight(ch + 1); l.Number != 2 {
		t.Fatalf("height %d before a Layout is line %d, want 2", ch+1, l.Number)
	}
}

func TestGeometryBeforeAnyLayoutIsZero(t *testing.T) {
	v, err := New("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	v.SetContent(Content{Text: "a\nb"})
	// Nothing has a height yet, so any height is below the document: its
	// last line.
	if top, bottom := v.LineBlock(1); top != 0 || bottom != 0 || v.ContentHeight() != 0 || v.LineAtHeight(10) != 1 {
		t.Fatalf("before any Layout: line 2 at %d..%d, height %d, line at 10 %d", top, bottom, v.ContentHeight(), v.LineAtHeight(10))
	}
}

func TestGeometryIsTheLastFramesGutterAndCells(t *testing.T) {
	v, err := New("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	if g := v.Geometry(); g != (Geometry{}) {
		t.Fatalf("geometry %+v before any Layout, want zero", g)
	}
	h := newHarness(t, strings.Repeat("line\n", 119))
	g := h.v.Geometry()
	if g.CellWidth != h.v.r.CellW() || g.GutterWidth != 5*g.CellWidth || g.TextX != g.GutterWidth+g.CellWidth/2 {
		t.Fatalf("geometry %+v, want a gutter of 3 digits and 2 cells, text half a cell past it", g)
	}
	h.v.SetContent(Content{Text: strings.Repeat("line\n", 1000)})
	h.frame()
	if got := h.v.Geometry().GutterWidth; got != 6*g.CellWidth {
		t.Fatalf("gutter %d with a fourth digit, want %d", got, 6*g.CellWidth)
	}
}

func TestAnEditorsGeometryIsItsViews(t *testing.T) {
	h := newHarness(t, "")
	h.ed = newTestEditor(t, EditorStateConfig{Doc: strings.Repeat("x\n", 20)})
	h.frame()
	if g := h.ed.Geometry(); g != h.ed.view.Geometry() || g.GutterWidth == 0 {
		t.Fatalf("editor geometry %+v, its view's %+v", g, h.ed.view.Geometry())
	}
}
