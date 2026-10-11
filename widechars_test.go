package godemirror

import (
	"slices"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
)

// "x中y": x is cell 0, 中 cells 1 and 2, y cell 3.
const wideSample = "x中y"

// atX is the point frac cells into line, with no scrolling.
func (h *harness) atX(line int, cells float32) f32.Point {
	cw := float32(h.v.r.CellW())
	p := h.at(line, 0)
	return f32.Pt(p.X-1+cells*cw, p.Y)
}

func TestAWideCharacterTakesTwoCells(t *testing.T) {
	h := newHarness(t, wideSample)
	h.click(h.at(0, 3), 0)
	h.click(h.at(0, 4), key.ModShift)
	if got := h.v.SelectedText(); got != "y" {
		t.Fatalf("cell 3 selected %q, want %q", got, "y")
	}
}

func TestAClickOnEitherHalfOfAWideCharacterLandsAroundIt(t *testing.T) {
	cases := []struct {
		at   float32
		want string
	}{
		{1.9, "x"},  // left half, nearer the middle
		{2.1, "x中"}, // right half, nearer the middle
		{1.2, "x"},
		{2.8, "x中"},
	}
	for _, c := range cases {
		h := newHarness(t, wideSample)
		h.click(h.at(0, 0), 0)
		h.click(h.atX(0, c.at), key.ModShift)
		if got := h.v.SelectedText(); got != c.want {
			t.Errorf("click at cell %.1f selected %q, want %q", c.at, got, c.want)
		}
		_, to, _ := h.v.Selection()
		if want := len([]rune(c.want)); to.Col != want {
			t.Errorf("click at cell %.1f ends at column %d, want %d", c.at, to.Col, want)
		}
	}
}

func TestDoubleClickSelectsAWordOfWideCharacters(t *testing.T) {
	h := newHarness(t, "ab 中文字 cd")
	// 文 is cells 5 and 6; its right half.
	h.click(h.atX(0, 6.5), 0)
	h.click(h.atX(0, 6.5), 0)
	if got := h.v.SelectedText(); got != "中文字" {
		t.Fatalf("double click selected %q, want %q", got, "中文字")
	}
}

func TestDoubleClickOnAWideSymbolSelectsItWhole(t *testing.T) {
	h := newHarness(t, " 😀 ")
	// Its left half: a press lands between characters, and a double click
	// takes the character after that place, as it does for a narrow one.
	h.click(h.atX(0, 1.4), 0)
	h.click(h.atX(0, 1.4), 0)
	if got := h.v.SelectedText(); got != "😀" {
		t.Fatalf("double click selected %q, want %q", got, "😀")
	}
}

func TestSelectCountsRunesAroundWideCharacters(t *testing.T) {
	h := newHarness(t, "\t中y\n文")
	h.v.Select(Pos{0, 1}, Pos{1, 1})
	if got, want := h.v.SelectedText(), "中y\n文"; got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	from, to, ok := h.v.Selection()
	if !ok || from != (Pos{0, 1}) || to != (Pos{1, 1}) {
		t.Fatalf("Selection = %v..%v %v, want {0 1}..{1 1}", from, to, ok)
	}
}

func TestASpanPastAWideCharacterCoversItsCells(t *testing.T) {
	h := newHarness(t, wideSample)
	if from, to := h.v.spanCells(0, Span{From: 1, To: 3}); from != 1 || to != 4 {
		t.Fatalf("span of 中y covers cells %d..%d, want 1..4", from, to)
	}
}

func TestWrapNeverSplitsAWideCharacter(t *testing.T) {
	cases := []struct {
		line string
		cols int
		want []int
	}{
		{"中文字", 3, []int{0, 2, 4}},
		{"a中文", 2, []int{0, 1, 3}},
		{"中文", 1, []int{0, 2}},
	}
	for _, c := range cases {
		if got := wrapLine(plain(c.line).lines[0], c.cols); !slices.Equal(got, c.want) {
			t.Errorf("wrapLine(%q, %d) = %v, want %v", c.line, c.cols, got, c.want)
		}
	}
}

func TestAWideGutterLabelTakesTwoCells(t *testing.T) {
	h := newHarness(t, "")
	h.v.SetContent(Content{Text: "a", Labels: []string{"中"}})
	if h.v.labelWidth != 2 {
		t.Fatalf("label width %d, want 2", h.v.labelWidth)
	}
}

// A row scrolled sideways is composed from a page boundary that may fall on a
// wide character's second half, which must still show that half.
func TestARowStartingInsideAWideCharacterDrawsItsSecondHalf(t *testing.T) {
	h := newHarness(t, wideSample)
	img := h.v.rasterize(h.v.doc.lines[0][2:], h.v.doc.classes[0][2:])
	for y := range img.Bounds().Dy() {
		for x := range h.v.r.CellW() {
			if img.RGBAAt(x, y).A != 0 {
				return
			}
		}
	}
	t.Fatal("the first cell, 中's second half, is blank")
}
