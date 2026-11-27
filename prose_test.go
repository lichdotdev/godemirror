package godemirror

import (
	"context"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
)

func TestTheWheelScrollsALongHoverWithinItsText(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: strings.Repeat("\n", 8) + "foo barbaz qux"})
	h.size.Y = 400
	long := strings.Repeat("A paragraph of documentation that wraps.\n\n", 60)
	h.ed.Hover = func(context.Context, EditorState, int) string { return long }
	h.move(h.at(8, 6), 0)
	h.advance(hoverDelay)
	h.until("shown", func() bool { return !h.ed.popups.boxes[hoverPopup].Empty() })
	box := h.ed.popups.boxes[hoverPopup]
	if box.Dy() > h.size.Y {
		t.Fatalf("the hover is %dpx tall in a %dpx editor", box.Dy(), h.size.Y)
	}
	wheel := func(dy float32) {
		h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: centre(box), Scroll: f32.Pt(0, dy)})
	}
	wheel(50)
	if got := h.ed.popups.scroll[hoverPopup]; got != 50 {
		t.Fatalf("scrolled %d, want 50", got)
	}
	wheel(1e5)
	end := h.ed.popups.scroll[hoverPopup]
	if end <= 50 || end > 60*h.ed.f.ch*4 {
		t.Fatalf("scrolled to %d, want the end of the text", end)
	}
	wheel(-1e6)
	if got := h.ed.popups.scroll[hoverPopup]; got != 0 {
		t.Fatalf("scrolled back to %d, want 0", got)
	}
	if h.v.ScrollTop() != 0 {
		t.Fatal("the wheel over the hover scrolled the code")
	}
}

func TestProseWrapsWithinItsWidth(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "x"})
	gtx := h.gtx()
	blocks := parseMarkdown(strings.Repeat("word ", 200) + "\n\n```go\nfunc F()\n```")
	p := h.ed.layoutProse(gtx, blocks, 300)
	if p.size.X > 300 {
		t.Fatalf("prose %dpx wide, want at most 300", p.size.X)
	}
	lines := map[int]bool{}
	code := 0
	for _, pc := range p.pieces {
		lines[pc.at.Y] = true
		if pc.code {
			code++
		}
	}
	if len(lines) < 5 || code == 0 {
		t.Fatalf("%d lines and %d code runs, want the words wrapped and the fence as code", len(lines), code)
	}
}

// gtx is a context for laying the editor's parts out, as its Layout does.
func (h *harness) gtx() layout.Context {
	h.ops.Reset()
	return layout.Context{Ops: &h.ops, Now: h.now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(h.size)}
}
