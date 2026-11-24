package godemirror

import (
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

// indented is a line that wraps into several rows in a 400px view.
var indented = "    " + strings.Repeat("word ", 40)

// rowPoint is a point on visual row row, cells cells into the text area,
// virtual indent included.
func (h *harness) rowPoint(row, cells int) f32.Point {
	return h.atX(0, float32(cells)).Add(f32.Pt(0, float32(row*h.v.r.CellH())))
}

func TestAWrappedRowHangsByTheLinesIndentationAndTwoCells(t *testing.T) {
	h := newHarness(t, indented)
	h.v.Wrap = true
	h.frame()
	hang := hangOf(h.v.doc.lines[0], h.v.wrapCols)
	if hang != 6 || len(h.v.wraps[0]) < 3 {
		t.Fatalf("hang %d over %d rows, want 6 over at least 3", hang, len(h.v.wraps[0]))
	}
	for i := 1; i < len(h.v.wraps[0]); i++ {
		from, to := h.v.segmentSpan(0, i)
		if to < len(indented) && to-from > h.v.wrapCols-hang {
			t.Fatalf("row %d holds %d cells, more than the %d it has room for", i, to-from, h.v.wrapCols-hang)
		}
	}
}

func TestPressingOnAContinuationRowCountsFromItsIndent(t *testing.T) {
	h := newHarness(t, indented)
	h.v.Wrap = true
	h.frame()
	from, _ := h.v.segmentSpan(0, 1)
	hang := h.v.hang(0, 1)
	for _, c := range []struct{ cells, col int }{
		{hang, from},
		{hang + 3, from + 3},
		{0, from},
		{hang - 1, from},
	} {
		h.pointer(pointer.Press, h.rowPoint(1, c.cells), 0)
		if got := h.v.sel.head; got != (pos{0, c.col}) {
			t.Fatalf("a press %d cells in landed at %v, want column %d", c.cells, got, c.col)
		}
		h.pointer(pointer.Release, h.rowPoint(1, c.cells), 0)
	}
}

func TestCopyingAcrossAContinuationRowLeavesTheVirtualIndentOut(t *testing.T) {
	h := newHarness(t, indented+"\nnext")
	h.v.Wrap = true
	h.frame()
	from, _ := h.v.segmentSpan(0, 1)
	hang := h.v.hang(0, 1)
	h.pointer(pointer.Press, h.rowPoint(0, 0), 0)
	h.pointer(pointer.Move, h.rowPoint(1, hang+4), 0)
	h.pointer(pointer.Release, h.rowPoint(1, hang+4), 0)
	if got, want := h.v.SelectedText(), indented[:from+4]; got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	h.v.Select(Pos{0, from}, Pos{1, 2})
	if got, want := h.v.SelectedText(), indented[from:]+"\nne"; got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestTheEditorsCaretAndCursorKeysFollowAHangingRow(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: indented})
	h.ed.Wrap = true
	h.frame()
	h.router.Source().Execute(key.FocusCmd{Tag: h.v})
	h.frame()
	from, _ := h.v.segmentSpan(0, 1)
	hang, cw := h.v.hang(0, 1), h.v.r.CellW()

	x, y := h.ed.coordsAt(from+2, 1)
	if want := (2 + hang) * cw; x != want || y != h.v.r.CellH() {
		t.Fatalf("a cell 2 into the row is at (%d, %d), want (%d, %d)", x, y, want, h.v.r.CellH())
	}

	h.click(h.rowPoint(1, hang+5), 0)
	if _, head := h.mainRange(); head != from+5 {
		t.Fatalf("a click 5 cells into the second row put the caret at %d, want %d", head, from+5)
	}
	h.press(key.NameDownArrow, 0)
	if _, head := h.mainRange(); head != h.v.wraps[0][2]+5 {
		t.Fatalf("Down went to %d, want 5 cells into the third row, %d", head, h.v.wraps[0][2]+5)
	}
	h.press(key.NameUpArrow, 0)
	h.press(key.NameUpArrow, 0)
	// The first row has no indent, so the column the caret keeps on screen
	// is hang cells further into its text.
	if _, head := h.mainRange(); head != 5+hang {
		t.Fatalf("Up twice went to %d, want the same x on the first row, %d", head, 5+hang)
	}
	h.press(key.NameHome, 0)
	h.press(key.NameDownArrow, 0)
	if _, head := h.mainRange(); head != from {
		t.Fatalf("Down from the start of the line went to %d, want the start of the row, %d", head, from)
	}
	h.press(key.NameEnd, 0)
	if _, head := h.mainRange(); head != h.v.wraps[0][2] {
		t.Fatalf("End went to %d, want the end of the second row, %d", head, h.v.wraps[0][2])
	}
}
