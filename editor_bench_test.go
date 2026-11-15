package godemirror

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// benchmarkKeystroke types a character in the middle of the 13,000-line
// Java file and lays the editor out, once per iteration: what a keystroke
// costs the UI goroutine, highlighting aside, which runs off it.
func benchmarkKeystroke(b *testing.B, wrap bool) {
	e, err := NewEditor("monospace", 12)
	if err != nil {
		b.Fatal(err)
	}
	e.Wrap = wrap
	text := bigJava(b)
	if err := e.SetState(EditorStateConfig{Filename: "Big.java", Doc: text}); err != nil {
		b.Fatal(err)
	}
	var ops op.Ops
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Ops: &ops, Now: time.Unix(1, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(1200, 900)),
		}
		if _, err := e.Layout(gtx); err != nil {
			b.Fatal(err)
		}
	}
	at := len(text) / 2
	if err := e.Dispatch(TransactionSpec{Selection: SingleSelection(at, at), ScrollIntoView: true}); err != nil {
		b.Fatal(err)
	}
	frame()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := e.Dispatch(e.State().ReplaceSelection("x")); err != nil {
			b.Fatal(err)
		}
		frame()
	}
}

func BenchmarkEditorKeystroke(b *testing.B)     { benchmarkKeystroke(b, false) }
func BenchmarkEditorKeystrokeWrap(b *testing.B) { benchmarkKeystroke(b, true) }
