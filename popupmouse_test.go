package godemirror

import (
	"context"
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

// centre is the middle of r as a pointer position.
func centre(r image.Rectangle) f32.Point {
	c := r.Min.Add(r.Max).Div(2)
	return f32.Pt(float32(c.X), float32(c.Y))
}

func TestClickingACompletionTakesIt(t *testing.T) {
	h, _ := completingEditor(t, "\n\n\nx.", "Relaunch", "Requested", "ReadyFile")
	h.typeText("Re")
	h.waitForCompletions()
	box := h.ed.popups.boxes[menuPopup]
	second := f32.Pt(float32(box.Min.X+10), float32(box.Min.Y+1+h.ed.menuRowH*3/2))
	h.click(second, 0)
	if got := h.ed.State().Doc(); got != "\n\n\nx.Requested" {
		t.Fatalf("doc %q after clicking the second item", got)
	}
}

func TestTheWheelScrollsALongMenu(t *testing.T) {
	var labels []string
	for i := range 20 {
		labels = append(labels, fmt.Sprintf("Re%02d", i))
	}
	h, _ := completingEditor(t, "x.", labels...)
	h.typeText("Re")
	h.waitForCompletions()
	at := centre(h.ed.popups.boxes[menuPopup])
	h.send(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: at, Scroll: f32.Pt(0, float32(3*h.ed.menuRowH))})
	if h.ed.completion.top != 3 {
		t.Fatalf("top %d after scrolling three rows, want 3", h.ed.completion.top)
	}
	if h.ed.State().Doc() != "x.Re" || h.v.ScrollTop() != 0 {
		t.Fatal("the wheel over the menu reached the code")
	}
}

func TestThePointerOverAHoverKeepsIt(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "\n\n\n\nfoo barbaz qux"})
	h.ed.Hover = func(context.Context, EditorState, int) HoverInfo { return HoverInfo{Text: "docs of barbaz"} }
	h.move(h.at(4, 6), 0)
	h.advance(hoverDelay)
	h.until("shown", func() bool { return !h.ed.popups.boxes[hoverPopup].Empty() })
	h.move(centre(h.ed.popups.boxes[hoverPopup]), 0)
	h.advance(time.Second)
	if !h.ed.hover.fired || h.ed.hover.info.Text == "" {
		t.Fatal("the hover closed as the pointer went into it")
	}
	h.click(h.at(4, 12), 0)
	if h.ed.hover.fired {
		t.Fatal("a click outside left the hover open")
	}
}

func TestAClickOutsideClosesTheMenu(t *testing.T) {
	h, _ := completingEditor(t, "x.\n\n", "Relaunch")
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(2, 2)})
	h.typeText("Re")
	h.waitForCompletions()
	h.click(h.at(0, 0), 0)
	if h.ed.completion.open {
		t.Fatal("the menu stayed open after a click in the code")
	}
	h.press(key.NameEscape, 0)
}
