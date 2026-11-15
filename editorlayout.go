package godemirror

import (
	"image"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const (
	// cursorBlinkRate is a whole blink of the caret, shown then hidden, as
	// @codemirror/view 6.43.15's drawSelection defaults it.
	cursorBlinkRate = 1200 * time.Millisecond
	// cursorWidth is the caret's width: CodeMirror's base theme draws it as
	// a 1.2px border.
	cursorWidth = unit.Dp(1.2)
)

// Layout draws the editor into the whole of gtx.Constraints.Max, as
// View.Layout draws a view; takes the keys, text, clipboard contents and
// pointer addressed to it and dispatches what they do; and then reports what
// was dispatched to OnUpdate. While it has focus, the caret of every range
// blinks, and Layout asks for a frame at each blink. The errors are
// View.Layout's.
func (e *Editor) Layout(gtx layout.Context) (layout.Dimensions, error) {
	v := e.view
	v.Wrap, v.Theme = e.Wrap, e.Theme
	e.now = gtx.Now
	defer func() { e.now = time.Time{} }()
	e.show()
	f, err := v.prepare(gtx)
	if err != nil {
		return layout.Dimensions{}, err
	}
	e.place(f)
	e.handleInput(gtx)
	v.handlePointer(gtx, e.f)
	if e.show() {
		e.place(e.f)
	}
	e.scrollIntoView(gtx)
	e.trackCrosshair()
	dims := v.paint(gtx, e.f)
	if gtx.Focused(v) {
		e.paintCarets(gtx)
		e.tellIME(gtx)
	}
	if v.scroll != v.scrolled {
		v.scrolled = v.scroll
		if e.OnScroll != nil {
			e.OnScroll()
		}
	}
	e.report(gtx)
	return dims, nil
}

// Focus gives the editor the keyboard, as CodeMirror's EditorView.focus: it
// takes keys and text from the next frame on, and shows its carets.
func (e *Editor) Focus(gtx layout.Context) { gtx.Execute(key.FocusCmd{Tag: e.view}) }

// place lays the rows of what the view shows out in f.
func (e *Editor) place(f frame) {
	v := e.view
	f.g = v.geometry(f.size, f.cw)
	v.wrapRows(f.g.cols)
	v.starts = rowStarts(v.wraps, nil)
	v.tops = lineTops(v.starts, f.ch, v.extra)
	v.laid = v.doc
	e.f = f
}

// show gives the view what changed since it last showed the state, and
// reports whether the document changed.
func (e *Editor) show() (edited bool) {
	edited = !e.unshown.Empty()
	if edited {
		fromA, toA := len(e.shown.doc), 0
		e.unshown.IterChanges(func(a, b, _, _ int, _ string) {
			fromA, toA = min(fromA, a), max(toA, b)
		})
		keepTop := e.shown.LineAt(fromA).Number - 1
		keepBottom := e.shown.Lines() - e.shown.LineAt(toA).Number
		first, last := e.state.Line(keepTop+1), e.state.Line(e.state.Lines()-keepBottom)
		e.view.showEdit(e.state.content(), keepTop, keepBottom, e.state.doc[first.From:last.To])
		e.unshown = emptyChangeSet(len(e.state.doc))
	} else if e.state.decosGen != e.shown.decosGen {
		e.view.content.Decorations, e.view.content.Widgets = e.state.lineDecorations()
	}
	if e.state.decosGen != e.shown.decosGen {
		e.shown.decos, e.shown.decosGen = e.state.decos, e.state.decosGen
	}
	if !edited && e.mark != nil && e.state.sel.eq(e.shown.sel, true) {
		return false
	}
	e.shown = e.state
	ranges := e.state.sel.ranges
	e.mark = make([]selection, len(ranges))
	for i, r := range ranges {
		e.mark[i] = selection{e.cellAt(r.Anchor()), e.cellAt(r.Head())}
	}
	e.view.sel = e.mark[e.state.sel.main]
	e.view.others = nil
	for i, s := range e.mark {
		if i != e.state.sel.main {
			e.view.others = append(e.view.others, s)
		}
	}
	return edited
}

// scrollIntoView scrolls the main range's head into view, as CodeMirror's
// scrollIntoView does by default, revealMargin clear of the edges; after a
// page motion, it keeps the head's row where it was on screen instead.
func (e *Editor) scrollIntoView(gtx layout.Context) {
	if !e.scrollPending {
		return
	}
	e.scrollPending = false
	v, f := e.view, e.f
	main := e.state.sel.Main()
	x, top := e.coordsAt(main.Head(), main.Assoc())
	if e.scrollTarget != nil {
		x, top = e.coordsAt(*e.scrollTarget, 0)
		e.scrollTarget = nil
	}
	margin := gtx.Dp(revealMargin)
	y := v.scroll.Y
	switch {
	case e.keepOnScreen != nil:
		y = top - *e.keepOnScreen
		e.keepOnScreen = nil
	case top-margin < y:
		y = top - margin
	case top+f.ch+margin > y+f.size.Y:
		y = top + f.ch + margin - f.size.Y
	}
	v.scroll.Y = min(max(y, 0), max(v.tops[len(v.tops)-1]-f.size.Y, 0))
	if v.Wrap {
		return
	}
	width := f.size.X - f.g.textX
	switch {
	case x-margin < v.scroll.X:
		v.scroll.X = max(x-margin, 0)
	case x+margin > v.scroll.X+width:
		v.scroll.X = x + margin - width
	}
}

// scrollX is how far the view is scrolled sideways: none when it wraps.
func (v *View) scrollX() int {
	if v.Wrap {
		return 0
	}
	return v.scroll.X
}

// paintCarets draws a caret at the head of every range in its blink's shown
// half, and asks for a frame when the blink turns.
func (e *Editor) paintCarets(gtx layout.Context) {
	if e.selectionSet || e.blinkStart.IsZero() {
		e.blinkStart, e.selectionSet = gtx.Now, false
	}
	half := cursorBlinkRate / 2
	phase := gtx.Now.Sub(e.blinkStart) % cursorBlinkRate
	next := half - phase
	if phase >= half {
		next = cursorBlinkRate - phase
	}
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(next)})
	if phase >= half {
		return
	}
	f := e.f
	defer clip.Rect(image.Rect(f.g.gutterW, 0, f.size.X, f.size.Y)).Push(gtx.Ops).Pop()
	for _, c := range e.carets(gtx) {
		paint.FillShape(gtx.Ops, e.Theme.Cursor, clip.Rect(c).Op())
	}
}

// carets are where the caret of each range shows, at its head.
func (e *Editor) carets(gtx layout.Context) []image.Rectangle {
	v, f := e.view, e.f
	w := max(gtx.Dp(cursorWidth), 1)
	var rects []image.Rectangle
	for _, r := range e.state.sel.ranges {
		x, top := e.coordsAt(r.Head(), r.Assoc())
		x += f.g.textX - v.scrollX() - w/2
		y := top - v.scroll.Y
		rects = append(rects, image.Rect(x, y, x+w, y+f.ch))
	}
	return rects
}
