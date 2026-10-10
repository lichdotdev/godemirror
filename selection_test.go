package godemirror

import (
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

const sample = "func main() {\n\tx := 1\n}"

func TestDragSelectsAcrossLinesWithTabsIntact(t *testing.T) {
	h := newHarness(t, sample)
	h.pointer(pointer.Press, h.at(0, 5), 0)
	h.pointer(pointer.Move, h.at(1, 6), 0)
	h.pointer(pointer.Release, h.at(1, 6), 0)
	if got, want := h.v.SelectedText(), "main() {\n\tx "; got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	if from, to, ok := h.v.SelectedLines(); !ok || from != 0 || to != 1 {
		t.Fatalf("lines = %d..%d %v, want 0..1", from, to, ok)
	}
}

func TestDoubleClickSelectsWordTripleClickTheLine(t *testing.T) {
	h := newHarness(t, sample)
	h.click(h.at(0, 6), 0)
	h.click(h.at(0, 6), 0)
	if got := h.v.SelectedText(); got != "main" {
		t.Fatalf("double click selected %q, want %q", got, "main")
	}
	h.click(h.at(0, 6), 0)
	if got, want := h.v.SelectedText(), "func main() {\n"; got != want {
		t.Fatalf("triple click selected %q, want %q", got, want)
	}
}

func TestSlowSecondClickStartsOver(t *testing.T) {
	h := newHarness(t, sample)
	h.click(h.at(0, 6), 0)
	h.now = h.now.Add(time.Second)
	h.click(h.at(0, 6), 0)
	if got := h.v.SelectedText(); got != "" {
		t.Fatalf("selected %q, want nothing", got)
	}
}

func TestShiftClickExtendsAndLineStartIsNotTakenIn(t *testing.T) {
	h := newHarness(t, sample)
	h.click(h.at(0, 0), 0)
	h.click(h.at(1, 0), key.ModShift)
	if got, want := h.v.SelectedText(), "func main() {\n"; got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	if from, to, ok := h.v.SelectedLines(); !ok || from != 0 || to != 0 {
		t.Fatalf("lines = %d..%d %v, want 0..0", from, to, ok)
	}
}

func TestSelectAllThenCopyWritesTheClipboard(t *testing.T) {
	h := newHarness(t, sample)
	h.click(h.at(0, 0), 0)
	h.send(key.Event{Name: "A", Modifiers: key.ModShortcut, State: key.Press})
	h.send(key.Event{Name: "C", Modifiers: key.ModShortcut, State: key.Press})
	mime, content, ok := h.router.WriteClipboard()
	if !ok || mime != "application/text" || string(content) != sample {
		t.Fatalf("clipboard = %q %q %v, want the whole document", mime, content, ok)
	}
}

func TestSelectCountsColumnsInRunes(t *testing.T) {
	h := newHarness(t, "\tfoo bar\nnext")
	h.v.Select(Pos{Line: 0, Col: 1}, Pos{Line: 0, Col: 4})
	if got := h.v.SelectedText(); got != "foo" {
		t.Fatalf("selected %q, want %q", got, "foo")
	}
	from, to, ok := h.v.Selection()
	if !ok || from != (Pos{0, 1}) || to != (Pos{0, 4}) {
		t.Fatalf("Selection() = %v, %v, %v; want {0 1}, {0 4}, true", from, to, ok)
	}
}

func TestSelectionReadsAMouseSelectionInRunes(t *testing.T) {
	h := newHarness(t, "\tfoo bar")
	word := h.at(0, 5)
	h.click(word, 0)
	h.click(word, 0)
	from, to, ok := h.v.Selection()
	if !ok || from != (Pos{0, 1}) || to != (Pos{0, 4}) {
		t.Fatalf("Selection() = %v, %v, %v; want {0 1}, {0 4}, true", from, to, ok)
	}
}

func TestSelectOrdersItsEnds(t *testing.T) {
	h := newHarness(t, "one\ntwo")
	h.v.Select(Pos{1, 2}, Pos{0, 1})
	if got := h.v.SelectedText(); got != "ne\ntw" {
		t.Fatalf("selected %q, want %q", got, "ne\ntw")
	}
}

func TestSelectClampsToTheDocument(t *testing.T) {
	h := newHarness(t, "one\ntwo")
	h.v.Select(Pos{1, 1}, Pos{9, 9})
	if got := h.v.SelectedText(); got != "wo" {
		t.Fatalf("selected %q, want %q", got, "wo")
	}
}

func TestSelectLinesRoundTripsThroughSelectedLines(t *testing.T) {
	h := newHarness(t, "a\nb\nc\nd")
	h.v.SelectLines(1, 2)
	from, to, ok := h.v.SelectedLines()
	if !ok || from != 1 || to != 2 {
		t.Fatalf("SelectedLines() = %d, %d, %v; want 1, 2, true", from, to, ok)
	}
	if got := h.v.SelectedText(); got != "b\nc\n" {
		t.Fatalf("selected %q, want %q", got, "b\nc\n")
	}
}

func TestSelectLinesTakesTheLastLineToItsEnd(t *testing.T) {
	h := newHarness(t, "a\nb")
	h.v.SelectLines(1, 1)
	if got := h.v.SelectedText(); got != "b" {
		t.Fatalf("selected %q, want %q", got, "b")
	}
}
