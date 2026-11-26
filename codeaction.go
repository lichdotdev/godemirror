package godemirror

import (
	"context"
	"image"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// CodeAction is an action CodeActions offers on a range: Title is what
// the list shows, Kind what it is ("quickfix", "refactor"...), and Data
// the hook's own, for ApplyCodeAction to read.
type CodeAction struct {
	Title, Kind string
	Data        any
}

// actionsState is the code actions on the cursor's line: asked for when the
// cursor reaches a line with a diagnostic, shown as a bulb in the gutter
// and, opened, as a list.
type actionsState struct {
	// line is the line (from 1) they were asked for, and gen the
	// diagnostics' generation then; 0 for none asked.
	line, gen int
	items     []CodeAction
	call      *call[[]CodeAction]
	// open is whether the list shows, or will once the answer comes, sel
	// the chosen action and at where the list points.
	open  bool
	sel   int
	at    int
	apply *call[renameAnswer]
}

// bulbSize is the size of the bulb in the gutter.
const bulbSize = unit.Dp(9)

// askActions asks CodeActions for the actions on the main selection, or
// the cursor's line when it is a cursor, with the diagnostics there; open
// shows the list once they come.
func (e *Editor) askActions(open bool) {
	hook := e.CodeActions
	if hook == nil {
		return
	}
	a := &e.actions
	main := e.state.sel.Main()
	line := e.state.LineAt(main.Head())
	from, to := main.from, main.to
	if from == to {
		from, to = line.From, line.To
	}
	var diags []Diagnostic
	for _, d := range e.diags {
		if d.From <= to && d.To >= from {
			diags = append(diags, d)
		}
	}
	a.call.stop()
	a.call = nil
	a.line, a.gen, a.open, a.sel, a.at = line.Number, e.diagsGen, open, 0, main.Head()
	s := e.state
	e.later(func() {
		a.call.stop()
		a.call = startCall(func(ctx context.Context) []CodeAction { return hook(ctx, s, from, to, diags) })
	})
}

// trackActions asks for the actions when the cursor reaches a line with a
// diagnostic, or the diagnostics change, and takes the answers.
func (e *Editor) trackActions(gtx layout.Context) {
	a := &e.actions
	line := e.state.LineAt(e.state.sel.Main().Head())
	if !a.open && (line.Number != a.line || e.diagsGen != a.gen) {
		a.items, a.line = nil, 0
		if e.lineHasDiagnostic(line) {
			e.askActions(false)
		}
	}
	if a.call != nil {
		if items, ok := a.call.poll(gtx); ok {
			a.call, a.items = nil, items
			a.open = a.open && len(items) > 0
		}
	}
	if a.apply != nil {
		if ans, ok := a.apply.poll(gtx); ok {
			a.apply = nil
			e.takeEdits(ans, a.at)
		}
	}
}

func (e *Editor) lineHasDiagnostic(l Line) bool {
	for _, d := range e.diags {
		if d.From <= l.To && d.To >= l.From {
			return true
		}
	}
	return false
}

// actionKey runs a key on the open list, and reports whether it took it.
func (e *Editor) actionKey(ev key.Event) bool {
	a := &e.actions
	if !a.open || ev.Modifiers != 0 {
		return false
	}
	n := len(a.items)
	switch ev.Name {
	case key.NameUpArrow:
		a.sel = (a.sel - 1 + n) % max(n, 1)
	case key.NameDownArrow:
		a.sel = (a.sel + 1) % max(n, 1)
	case key.NameReturn, key.NameEnter, key.NameTab:
		e.takeAction(a.sel)
	case key.NameEscape:
		a.open = false
	default:
		return false
	}
	return true
}

// takeAction applies action i: ApplyCodeAction's edits are made as a
// rename's are.
func (e *Editor) takeAction(i int) {
	a, hook := &e.actions, e.ApplyCodeAction
	a.open = false
	if hook == nil || i < 0 || i >= len(a.items) {
		return
	}
	action, s := a.items[i], e.state
	e.later(func() {
		a.apply.stop()
		a.apply = startCall(func(ctx context.Context) renameAnswer {
			edits, err := hook(ctx, s, action)
			return renameAnswer{edits, err, s.doc}
		})
	})
}

// takeEdits applies edits a hook made for a document that has not changed
// since, or shows its error at pos.
func (e *Editor) takeEdits(a renameAnswer, pos int) {
	if a.err != nil {
		e.rename.err, e.rename.at = a.err.Error(), pos
		return
	}
	if a.doc == e.state.doc {
		e.applyEdits(a.edits, "input.codeaction")
	}
}

// paintBulb draws the bulb in the first cell of the gutter on the line the
// actions are for, when there are some, and takes a press on it.
func (e *Editor) paintBulb(gtx layout.Context) {
	a := &e.actions
	if len(a.items) == 0 || a.line == 0 || a.line > e.state.Lines() {
		return
	}
	v, f := e.view, e.f
	top := v.tops[a.line-1] - v.scroll.Y
	if top+f.ch < 0 || top > f.size.Y {
		return
	}
	size := gtx.Dp(bulbSize)
	cx, cy := f.cw/2+1, top+f.ch/2-size/6
	glass := image.Rect(cx-size/2, cy-size/2, cx-size/2+size, cy-size/2+size)
	base := image.Rect(cx-size/4, glass.Max.Y-1, cx+size/4+1, glass.Max.Y+size/3)
	c := e.Theme.Diagnostics.Warning
	paint.FillShape(gtx.Ops, c, clip.Ellipse(glass).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, c, clip.Rect(base).Op())
	e.popupHandler(gtx, bulbPopup, image.Rect(0, top, f.g.gutterW-f.cw, top+f.ch))
}

// paintActions draws the list of actions under the cursor.
func (e *Editor) paintActions(gtx layout.Context) {
	a := &e.actions
	if !a.open || len(a.items) == 0 {
		return
	}
	p, f := e.Theme.Popup, e.f
	rowH := f.ch + 2*gtx.Dp(completionRowPad)
	padX := gtx.Dp(popupPadX)
	cols := 0
	for _, it := range a.items {
		cols = max(cols, len(textCells(it.Title)))
	}
	size := image.Pt(min(cols*f.cw+2*padX, f.size.X-f.g.gutterW), len(a.items)*rowH+2)
	r := e.placePopup(gtx, e.posBox(a.at), size, false)
	e.paintPopupBox(gtx, r)
	for i, it := range a.items {
		row := image.Rect(r.Min.X+1, r.Min.Y+1+i*rowH, r.Max.X-1, r.Min.Y+1+(i+1)*rowH)
		fg := p.Foreground
		if i == a.sel {
			paint.FillShape(gtx.Ops, p.Selected, clip.UniformRRect(row, gtx.Dp(popupRadius)).Op(gtx.Ops))
			fg = p.SelectedForeground
		}
		at := image.Pt(r.Min.X+padX, row.Min.Y+(rowH-f.ch)/2)
		e.view.paintTextSpans(gtx, []textSpan{{text: it.Title, color: fg}}, at, row)
	}
	e.actionRowH = rowH
	e.popupHandler(gtx, actionsPopup, r)
}

// pressActions takes the action at at, a point in the list.
func (e *Editor) pressActions(at image.Point) {
	box := e.popups.boxes[actionsPopup]
	if e.actionRowH > 0 {
		e.takeAction((at.Y - box.Min.Y - 1) / e.actionRowH)
	}
}
