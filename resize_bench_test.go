package godemirror

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// benchDiffLines is the size of one diff card in a review list.
const benchDiffLines = 60

func benchDiffView(b *testing.B, wrap bool) *DiffView {
	b.Helper()
	d, err := NewDiffView("monospace", 12, DiffOptions{})
	if err != nil {
		b.Fatal(err)
	}
	d.Wrap = wrap
	var old, cur strings.Builder
	for i := range benchDiffLines {
		line := fmt.Sprintf("\tresult = append(result, compute(input[%d], options.Threshold*%d)) // step %d\n", i, i, i)
		old.WriteString(line)
		if i%7 == 0 {
			line = strings.Replace(line, "compute", "computeFaster", 1)
		}
		cur.WriteString(line)
	}
	d.SetDocs("main.go", old.String(), cur.String())
	return d
}

// benchmarkDrag lays a DiffView out once per iteration at a width that
// changes every frame, as dragging the edge of the pane holding it does.
func benchmarkDrag(b *testing.B, wrap bool, step int) {
	d := benchDiffView(b, wrap)
	var ops op.Ops
	layoutAt := func(width int) {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Now:         time.Unix(1, 0),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(width, 1200)),
		}
		if _, err := d.Layout(gtx); err != nil {
			b.Fatal(err)
		}
	}
	for range 2 {
		layoutAt(600)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		layoutAt(600 + (i%200)*step)
	}
}

func BenchmarkDiffViewFixedWidth(b *testing.B)       { benchmarkDrag(b, false, 0) }
func BenchmarkDiffViewDraggedWidth(b *testing.B)     { benchmarkDrag(b, false, 3) }
func BenchmarkDiffViewDraggedWidthWrap(b *testing.B) { benchmarkDrag(b, true, 3) }
