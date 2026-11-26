package godemirror

import (
	"context"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
)

// glideFrames lays the editor out a frame each few milliseconds through a
// glide, calling check after each.
func (h *harness) glideFrames(check func()) {
	h.t.Helper()
	for range glideDuration/(10*time.Millisecond) + 2 {
		h.advance(10 * time.Millisecond)
		check()
	}
}

func TestTheCompletionMenuFollowsTheViewWhileItGlides(t *testing.T) {
	doc := strings.Repeat("line\n", 8) + "x."
	h, _ := completingEditor(t, doc+strings.Repeat("\nline", 100), "Relaunch", "Requested")
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(len(doc), len(doc)), ScrollIntoView: true})
	h.settle() // the text input learns the selection, and the view lands
	h.typeText("Re")
	h.waitForCompletions()
	gap := func() int {
		return h.ed.popups.boxes[menuPopup].Min.Y - h.ed.posBox(h.ed.state.sel.Main().Head()).Max.Y
	}
	want, before := gap(), h.v.ScrollTop()
	h.ed.SetScrollTop(before + 5*h.ed.f.ch)
	moved := false
	h.glideFrames(func() {
		if !h.ed.completion.open {
			t.Fatal("the glide closed the menu")
		}
		if got := gap(); got != want {
			t.Fatalf("at scroll %d the menu is %dpx under its row, want %d", h.v.ScrollTop(), got, want)
		}
		moved = moved || h.v.ScrollTop() != before
	})
	if !moved {
		t.Fatal("the view never glided")
	}
}

func TestTheHoverFollowsItsNameWhileTheViewGlides(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: strings.Repeat("alpha beta\n", 200)}, SingleSelection(0, 0))
	h.ed.Hover = func(context.Context, EditorState, int) string { return "docs" }
	h.move(h.at(8, 7), 0)
	h.advance(hoverDelay)
	h.until("shown", func() bool { return !h.ed.popups.boxes[hoverPopup].Empty() })
	name := h.ed.hover.name
	gap := func() (int, bool) {
		boxes := h.ed.rangeBoxes(name.from, name.to)
		if len(boxes) == 0 {
			return 0, false
		}
		return boxes[0].Min.Y - h.ed.popups.boxes[hoverPopup].Max.Y, true
	}
	want, _ := gap()
	before := h.v.ScrollTop()
	h.press(key.NamePageDown, 0)
	followed := 0
	h.glideFrames(func() {
		if got, ok := gap(); ok && h.ed.hover.fired {
			if got != want {
				t.Fatalf("at scroll %d the hover is %dpx over its name, want %d", h.v.ScrollTop(), got, want)
			}
			followed++
		}
	})
	if h.v.ScrollTop() == before || followed == 0 {
		t.Fatalf("scroll %d from %d, %d frames followed: want a glide the hover followed", h.v.ScrollTop(), before, followed)
	}
}
