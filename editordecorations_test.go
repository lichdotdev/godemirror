package godemirror

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
)

var resolved = LineDecoration{Background: color.NRGBA{G: 0x80, A: 0xff}}

func isResolved(d LineDecoration) bool { return d.Background == resolved.Background }

func TestALineDecorationFollowsItsLineThroughEdits(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "a\nb\nc", Decorations: Decorations{Lines: []LineDecorationAt{{Pos: 2, Decoration: resolved}}}})
	if !isResolved(h.v.decoration(1)) {
		t.Fatal("line 2 is not decorated")
	}
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(0, "new\n")}})
	h.frame()
	if isResolved(h.v.decoration(1)) || !isResolved(h.v.decoration(2)) {
		t.Fatal("the decoration did not move down with its line")
	}
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(6, "x")}})
	h.frame()
	if !isResolved(h.v.decoration(2)) {
		t.Fatal("text typed at the line's start moved the decoration off it")
	}
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{del(5, 6)}})
	h.frame()
	if len(h.ed.State().Decorations().Lines) != 0 || isResolved(h.v.decoration(1)) {
		t.Fatal("joining the line to the one above kept its decoration")
	}
}

func TestAWidgetFollowsItsPositionAndGoesWithIt(t *testing.T) {
	laid := 0
	w := func(gtx layout.Context) layout.Dimensions { laid++; return layout.Dimensions{Size: image.Pt(5, 5)} }
	h := newEditorHarness(t, EditorStateConfig{Doc: "a\nbc\nd", Decorations: Decorations{Widgets: []LineWidgetAt{{Pos: 3, Widget: w}}}})
	if laid == 0 {
		t.Fatal("the widget was not laid out")
	}
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(0, "\n")}})
	h.frame()
	if got := h.v.content.Widgets; len(got) != 1 || got[0].Line != 2 {
		t.Fatalf("widgets %+v, want one on line 3", got)
	}
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{del(3, 5)}})
	h.frame()
	if len(h.ed.State().Decorations().Widgets) != 0 || len(h.v.content.Widgets) != 0 {
		t.Fatal("a deletion across the widget kept it")
	}
}

func TestDispatchReplacesTheDecorations(t *testing.T) {
	h := newEditorHarness(t, EditorStateConfig{Doc: "a\nb"})
	d := Decorations{Lines: []LineDecorationAt{{Pos: 0, Decoration: resolved}}}
	dispatch(t, h.ed, TransactionSpec{Changes: []ChangeSpec{ins(0, "x\n")}, Decorations: &d})
	h.frame()
	if !isResolved(h.v.decoration(0)) || isResolved(h.v.decoration(1)) {
		t.Fatal("the decorations given are not at their positions in the new document")
	}
	dispatch(t, h.ed, TransactionSpec{Decorations: &Decorations{}})
	h.frame()
	if isResolved(h.v.decoration(0)) {
		t.Fatal("decorations replaced with no change to the document still show")
	}
	if err := h.ed.Dispatch(TransactionSpec{Decorations: &Decorations{Lines: []LineDecorationAt{{Pos: 1}}}}); err == nil {
		t.Fatal("a line decoration off a line's start was taken")
	}
}

func TestLineDecorationsOnOneLineMerge(t *testing.T) {
	strip := LineDecoration{GutterStrip: color.NRGBA{R: 0xff, A: 0xff}, Spans: []Span{{From: 0, To: 1}}}
	bg := LineDecoration{Background: resolved.Background, Spans: []Span{{From: 1, To: 2}}}
	h := newEditorHarness(t, EditorStateConfig{Doc: "ab", Decorations: Decorations{Lines: []LineDecorationAt{{0, strip}, {0, bg}}}})
	got := h.v.decoration(0)
	if got.GutterStrip != strip.GutterStrip || got.Background != bg.Background || len(got.Spans) != 2 {
		t.Fatalf("merged %+v, want both decorations' fields and spans", got)
	}
}
