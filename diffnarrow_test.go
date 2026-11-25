package godemirror

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
)

func TestUnifiedGutterNumbersRowsByTheFileTheyAreIn(t *testing.T) {
	for _, tc := range []struct {
		name     string
		gutter   UnifiedGutter
		old, new string
		want     string
		width    int
	}{
		{"two columns", TwoColumnGutter, "a\nb", "x\ny\nz", "1  |2  |  1|  2|  3", 3},
		{"one column", OneColumnGutter, "a\nb", "x\ny\nz", "1|2|1|2|3", 1},
		{"one column past a change", OneColumnGutter, "a\nb\nc\nd", "a\nB\nc\nd\ne", "1|2|2|3|4|5", 1},
		{"one column, wide numbers", OneColumnGutter, strings.Join(fileLines(12), "\n"), strings.Join(fileLines(12), "\n") + "\nx", "", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, d := newDiffHarness(t, DiffOptions{UnifiedGutter: tc.gutter})
			d.SetDocs("main.go", tc.old, tc.new)
			h.frame()
			var labels []string
			for i := range h.v.LineCount() {
				labels = append(labels, h.v.label(i))
			}
			if got := strings.Join(labels, "|"); tc.want != "" && got != tc.want {
				t.Fatalf("labels = %q, want %q", got, tc.want)
			}
			if h.v.labelWidth != tc.width {
				t.Fatalf("gutter is %d cells wide, want %d", h.v.labelWidth, tc.width)
			}
		})
	}
}

// gutterRevert is a diff with one change, whose revert sits in the gutter.
func gutterRevert(t *testing.T, split bool, reverted *[]Chunk) (*harness, *DiffView) {
	t.Helper()
	h, d := newDiffHarness(t, DiffOptions{
		UnifiedGutter:   OneColumnGutter,
		RevertPlacement: RevertInGutter,
		Revert:          func(c Chunk) { *reverted = append(*reverted, c) },
	})
	d.Split = split
	d.SetDocs("main.go", "a\nb\nc\n", "a\nB\nc\n")
	h.frame()
	return h, d
}

func TestRevertInTheGutterReplacesTheNumberWhileHovered(t *testing.T) {
	for _, split := range []bool{false, true} {
		t.Run(map[bool]string{false: "unified", true: "split"}[split], func(t *testing.T) {
			var reverted []Chunk
			h, d := gutterRevert(t, split, &reverted)
			v, x := d.unified, float32(2)
			if split {
				v = d.split.Right
				x += float32(h.size.X/2) + 1
			}
			if len(v.widgetAreas) != 0 {
				t.Fatalf("revert drawn with the pointer away: %v", v.widgetAreas)
			}
			// The first row of the chunk: row 1 unified (the deletion), row 1 split.
			y := h.at(1, 0).Y
			h.pointer(pointer.Move, f32.Pt(x, y), 0)
			if len(v.widgetAreas) != 1 || v.widgetAreas[0].Max.X > v.laidOut.GutterWidth {
				t.Fatalf("revert areas %v, want one inside the %dpx gutter", v.widgetAreas, v.laidOut.GutterWidth)
			}
			area := v.widgetAreas[0]
			mid := f32.Pt(x-2+float32(area.Min.X+area.Dx()/2), float32(area.Min.Y+area.Dy()/2))
			h.click(mid, 0)
			h.frame()
			if len(reverted) != 1 || !chunkEqual(reverted[0], d.Chunks()[0]) {
				t.Fatalf("reverted %+v, want the chunk", reverted)
			}
		})
	}
}

func TestAGutterWidgetHidesTheLabelItCovers(t *testing.T) {
	h := newHarness(t, "")
	h.v.SetContent(Content{
		Text:        "one\ntwo\nthree",
		Decorations: []LineDecoration{{HoverGroup: 1}, {HoverGroup: 1}, {}},
		Widgets: []LineWidget{{Line: 0, Placement: GutterCell, WhileHovered: true, Widget: func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(4, 4)}
		}}},
	})
	h.pointer(pointer.Move, f32.Pt(2, h.at(1, 0).Y), 0)
	if _, ok := h.v.gutter[gutterKey{0, "1"}]; ok {
		t.Fatal("the number under the widget was drawn")
	}
	if _, ok := h.v.gutter[gutterKey{1, "2"}]; !ok {
		t.Fatal("the number beside it was not")
	}
	h.pointer(pointer.Leave, f32.Pt(2, h.at(1, 0).Y), 0)
	if _, ok := h.v.gutter[gutterKey{0, "1"}]; !ok {
		t.Fatal("the number did not come back with the pointer gone")
	}
}

func TestADefaultRevertIsRightEdgeUnlessTheCallerChoosesTheGutter(t *testing.T) {
	for _, tc := range []struct {
		placement RevertPlacement
		want      Placement
	}{{RevertAtRightEdge, RightEdge}, {RevertInGutter, GutterCell}} {
		_, d := newDiffHarness(t, DiffOptions{RevertPlacement: tc.placement, Revert: func(Chunk) {}})
		d.SetDocs("main.go", "a\nb", "a\nB")
		var got []Placement
		for _, w := range d.widgets(d.unifiedSide, d.unified) {
			got = append(got, w.Placement)
		}
		if len(got) != 1 || got[0] != tc.want {
			t.Fatalf("placement %d: widgets at %v, want %v", tc.placement, got, tc.want)
		}
	}
}

// laidOutHeight is the height a diff takes when laid out in width pixels, read off
// the views themselves.
func laidOutHeight(h *harness, d *DiffView, width int) int {
	h.size = image.Pt(width, 300)
	h.frame()
	if d.Split {
		return d.split.Left.tops[len(d.split.Left.tops)-1]
	}
	return d.unified.tops[len(d.unified.tops)-1]
}

func TestHeightIsTheHeightLayoutTakes(t *testing.T) {
	long := strings.Repeat("word ", 30)
	// Continuation rows hang by the line's own indentation, so lines
	// indented differently on the two sides, and one indented past half a
	// row, wrap into different row counts.
	deep := strings.Repeat(" ", 60)
	old := "a\n\tindent " + long + "\n日本語日本語日本語日本語日本語日本語日本語日本語 " + long + "\nc\n" + strings.Repeat("x", 90) + "\n    " + long + "\n" + deep + long + "\n"
	updated := "a\nB\n\tindent " + long + "\nc\n" + long + long + "\n" + long + "\n" + deep + deep + long + "\n"
	for _, tc := range []struct {
		name        string
		split, wrap bool
	}{{"unified", false, false}, {"unified wrapped", false, true}, {"split", true, false}, {"split wrapped", true, true}} {
		for _, width := range []int{150, 333, 800} {
			t.Run(tc.name, func(t *testing.T) {
				h, d := newDiffHarness(t, DiffOptions{Collapse: defaultCollapse})
				d.Split, d.Wrap = tc.split, tc.wrap
				d.SetDocs("main.go", old, updated)
				gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
				got, err := d.Height(gtx, width)
				if err != nil {
					t.Fatal(err)
				}
				if want := laidOutHeight(h, d, width); got != want {
					t.Fatalf("Height(%d) = %d, laid out it takes %d", width, got, want)
				}
				if d.Wrap && width == 150 {
					d.Wrap = false
					if flat, _ := d.Height(gtx, width); got <= flat {
						t.Fatalf("wrapped height %d is no taller than unwrapped %d: nothing wrapped", got, flat)
					}
				}
			})
		}
	}
}

func TestHeightCountsWhatWasFilledIntoAGap(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{Collapse: defaultCollapse})
	collapsedDocs(d)
	h.frame()
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	before, _ := d.Height(gtx, 400)
	for _, g := range d.unifiedSide.gaps {
		d.FillGap(*g, d.src.newLines[g.From-1:g.To])
		break
	}
	for deadline := time.Now().Add(5 * time.Second); d.unified.pending != nil; {
		if time.Now().After(deadline) {
			t.Fatal("highlighting never landed")
		}
		time.Sleep(time.Millisecond)
		h.frame()
	}
	after, _ := d.Height(gtx, 400)
	if after <= before {
		t.Fatalf("height %d after filling a gap, %d before", after, before)
	}
	if want := laidOutHeight(h, d, 400); after != want {
		t.Fatalf("Height = %d, laid out it takes %d", after, want)
	}
}
