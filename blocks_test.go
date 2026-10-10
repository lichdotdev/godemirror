package godemirror

import (
	"image"
	"slices"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
)

// fixedBlock lays out height pixels and counts how many times it was laid out.
func fixedBlock(height int, layouts *int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		*layouts++
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, height)}
	}
}

func TestABlockPushesTheLinesUnderItDown(t *testing.T) {
	var layouts int
	h := newHarness(t, "")
	h.v.SetContent(Content{Text: "one\ntwo\nthree", Blocks: []BlockWidget{{After: 0, Widget: fixedBlock(30, &layouts)}}})
	h.frame()
	ch := h.v.r.CellH()
	if want := []int{0, ch + 30, 2*ch + 30, 3*ch + 30}; !slices.Equal(h.v.tops, want) {
		t.Fatalf("tops = %v, want %v", h.v.tops, want)
	}
	before := layouts
	h.frame()
	if layouts-before != 1 {
		t.Fatalf("block laid out %d times in a frame, want 1", layouts-before)
	}
	// A press below the block lands on "two", not on the block's own rows.
	two := h.at(1, 1)
	two.Y += 30
	h.click(two, 0)
	h.click(two, 0)
	if got := h.v.SelectedText(); got != "two" {
		t.Fatalf("double click under the block selected %q, want %q", got, "two")
	}
}

func TestPressInABlockSelectsNothing(t *testing.T) {
	var layouts int
	h := newHarness(t, "")
	h.v.SetContent(Content{Text: "one\ntwo", Blocks: []BlockWidget{{After: 0, Widget: fixedBlock(30, &layouts)}}})
	h.frame()
	inBlock := f32.Pt(h.at(0, 2).X, float32(h.v.r.CellH()+15))
	two := h.at(1, 2)
	two.Y += 30
	h.pointer(pointer.Press, inBlock, 0)
	h.pointer(pointer.Move, two, 0)
	h.pointer(pointer.Release, two, 0)
	if got := h.v.SelectedText(); got != "" {
		t.Fatalf("drag from inside the block selected %q", got)
	}
	if h.v.hover != 1 {
		t.Fatalf("hover = %d, want the line under the pointer", h.v.hover)
	}
}

func TestSplitLeavesRoomForTheOtherSidesBlock(t *testing.T) {
	split, err := NewSplit("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	var layouts int
	split.Left.SetContent(Content{Text: "a\nb\nc"})
	split.Right.SetContent(Content{Text: "a\nb\nc", Blocks: []BlockWidget{{After: 1, Widget: fixedBlock(40, &layouts)}}})
	h := &harness{t: t, v: split.Left, split: split, size: image.Pt(800, 200), now: time.Unix(1, 0)}
	h.frame()
	if !slices.Equal(split.Left.tops, split.Right.tops) {
		t.Fatalf("left tops %v, right tops %v, want them level", split.Left.tops, split.Right.tops)
	}
	ch := split.Left.r.CellH()
	if got, want := split.Left.tops[2], 2*ch+40; got != want {
		t.Fatalf("left line c starts at %d, want %d", got, want)
	}
}
