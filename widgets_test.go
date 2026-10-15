package godemirror

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror/tags"
)

// button is a clickable that counts its clicks, as a widget reads them:
// during its own layout, before laying out.
type button struct {
	widget.Clickable
	clicks int
}

// blocks is a document whose lines 1 and 2 are block 1, with a button that
// shows while that block is hot, and a plain one on line 0.
func blocks(t *testing.T, plainBtn, hotBtn *button) Content {
	t.Helper()
	widgetOf := func(c *button) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			for c.Clicked(gtx) {
				c.clicks++
			}
			return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(40, gtx.Constraints.Max.Y)}
			})
		}
	}
	return Content{
		Text: "context\nold\nnew\ncontext",
		Decorations: []LineDecoration{
			{},
			{HoverGroup: 1, HoverBackground: red500},
			{HoverGroup: 1, HoverBackground: red500},
		},
		Widgets: []LineWidget{
			{Line: 0, Placement: AfterText, Widget: widgetOf(plainBtn)},
			{Line: 1, Placement: RightEdge, WhileHovered: true, Widget: widgetOf(hotBtn)},
		},
	}
}

func TestHoverLightsTheBlockAndShowsItsWidget(t *testing.T) {
	var plainBtn, hotBtn button
	h := newHarness(t, "")
	h.v.SetContent(blocks(t, &plainBtn, &hotBtn))
	h.frame()
	if h.v.hot != 0 || len(h.v.widgetAreas) != 1 {
		t.Fatalf("cold: hot %d, %d widgets drawn, want 0 and 1", h.v.hot, len(h.v.widgetAreas))
	}
	h.pointer(pointer.Move, h.at(2, 1), 0)
	if h.v.hot != 1 || len(h.v.widgetAreas) != 2 {
		t.Fatalf("over block: hot %d, %d widgets drawn, want 1 and 2", h.v.hot, len(h.v.widgetAreas))
	}
	h.pointer(pointer.Leave, h.at(2, 1), 0)
	if h.v.hot != 0 {
		t.Fatalf("after leaving: hot %d, want 0", h.v.hot)
	}
}

func TestPressOnAWidgetClicksItAndSelectsNothing(t *testing.T) {
	var plainBtn, hotBtn button
	h := newHarness(t, "")
	h.v.SetContent(blocks(t, &plainBtn, &hotBtn))
	h.frame()
	area := h.v.widgetAreas[0]
	mid := f32.Pt(float32(area.Min.X+area.Dx()/2), float32(area.Min.Y+area.Dy()/2))
	h.pointer(pointer.Press, mid, 0)
	h.pointer(pointer.Release, mid, 0)
	h.frame()
	if plainBtn.clicks != 1 {
		t.Fatalf("button clicked %d times, want 1", plainBtn.clicks)
	}
	if !h.v.sel.empty() || h.v.sel.anchor != (pos{}) {
		t.Fatalf("press on the widget selected %+v", h.v.sel)
	}
}

func TestUpdateContentKeepsScrollAndSwapsWhole(t *testing.T) {
	h := newHarness(t, "")
	long := Content{Filename: "a.go", Text: "package a\n" + lines(200)}
	h.v.SetContent(long)
	h.frame()
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(200, 100), Scroll: f32.Pt(0, 300)})
	scrolled := h.v.scroll
	next := long
	next.Text = "package a\n" + lines(250)
	next.Labels = []string{"x"}
	h.v.UpdateContent(next)
	for deadline := time.Now().Add(5 * time.Second); h.v.pending != nil; {
		if time.Now().After(deadline) {
			t.Fatal("highlighting never landed")
		}
		time.Sleep(time.Millisecond)
		h.frame()
	}
	if h.v.scroll != scrolled {
		t.Fatalf("scroll %v, want it kept at %v", h.v.scroll, scrolled)
	}
	if len(h.v.doc.lines) != 251 || h.v.label(0) != "x" || h.v.doc.tags[0][0] != tags.ModuleKeyword {
		t.Fatalf("content not swapped whole: %d lines, label %q, tag %v", len(h.v.doc.lines), h.v.label(0), h.v.doc.tags[0][0])
	}
}

func lines(n int) string {
	b := make([]byte, 0, n*4)
	for range n {
		b = append(b, "var\n"...)
	}
	return string(b)
}
