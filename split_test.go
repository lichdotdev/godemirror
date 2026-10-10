package godemirror

import (
	"image"
	"slices"
	"testing"
	"time"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestPairedRowsTakesTheTallerSide(t *testing.T) {
	left := [][]int{{0}, {0, 10, 20}, {0}}
	right := [][]int{{0, 8}, {0}, {0}, {0, 5}}
	if got, want := pairedRows(left, right), []int{2, 3, 1, 2}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSelectingOnOneSideClearsTheOther(t *testing.T) {
	split, err := NewSplit("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	split.Left.SetContent(Content{Text: sample})
	split.Right.SetContent(Content{Text: sample})
	h := &harness{t: t, v: split.Left, split: split, size: image.Pt(800, 200), now: time.Unix(1, 0)}
	h.frame()
	leftWord := h.at(0, 6)
	h.click(leftWord, 0)
	h.click(leftWord, 0)
	if got := split.Left.SelectedText(); got != "main" {
		t.Fatalf("left selected %q, want %q", got, "main")
	}
	rightWord := leftWord
	rightWord.X += float32(800 / 2)
	h.now = h.now.Add(time.Second)
	h.click(rightWord, 0)
	h.click(rightWord, 0)
	if got := split.Right.SelectedText(); got != "main" {
		t.Fatalf("right selected %q, want %q", got, "main")
	}
	if got := split.Left.SelectedText(); got != "" {
		t.Fatalf("left still selected %q", got)
	}
}

func TestHoverOnOneSideLightsTheBlockOnBoth(t *testing.T) {
	split, err := NewSplit("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	c := Content{
		Text:        "same\nold\nsame",
		Decorations: []LineDecoration{{}, {HoverGroup: 7}},
	}
	split.Left.SetContent(c)
	split.Right.SetContent(c)
	h := &harness{t: t, v: split.Left, split: split, size: image.Pt(800, 200), now: time.Unix(1, 0)}
	h.frame()
	h.pointer(pointer.Move, h.at(1, 1), 0)
	if split.Left.hot != 7 || split.Right.hot != 7 {
		t.Fatalf("hot = %d left, %d right, want 7 on both", split.Left.hot, split.Right.hot)
	}
}

func TestSetThemeGivesBothSidesTheTheme(t *testing.T) {
	split, err := NewSplit("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	split.SetTheme(LightTheme)
	if split.Left.Theme != LightTheme || split.Right.Theme != LightTheme {
		t.Fatal("a side kept its old theme")
	}
}

func TestSplitRedrawsBothSidesAtANewScale(t *testing.T) {
	split, err := NewSplit("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	split.Left.SetContent(Content{Text: sample})
	split.Right.SetContent(Content{Text: sample})
	var ops op.Ops
	for _, scale := range []float32{1, 2} {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Now:         time.Unix(1, 0),
			Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
			Constraints: layout.Exact(image.Pt(800, 200)),
		}
		if _, err := split.Layout(gtx); err != nil {
			t.Fatal(err)
		}
	}
	for side, v := range map[string]*View{"left": split.Left, "right": split.Right} {
		for k, img := range v.rows {
			if img.size.Y != 0 && img.size.Y != v.r.CellH() {
				t.Fatalf("%s row %v is %dpx tall, want %d", side, k, img.size.Y, v.r.CellH())
			}
		}
		for line, img := range v.gutter {
			if img.size.Y != v.r.CellH() {
				t.Fatalf("%s gutter label %d is %dpx tall, want %d", side, line, img.size.Y, v.r.CellH())
			}
		}
	}
}
