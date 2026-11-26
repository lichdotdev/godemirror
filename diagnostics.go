package godemirror

import (
	"cmp"
	"image"
	"image/color"
	"slices"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Severity is how bad a Diagnostic is, from SeverityError, the worst, to
// SeverityHint, numbered as LSP numbers them.
type Severity int

// The severities of a diagnostic. The zero Severity, in NextDiagnostic and
// PrevDiagnostic, stands for any.
const (
	SeverityError Severity = iota + 1
	SeverityWarning
	SeverityInfo
	SeverityHint
)

// Diagnostic is a problem in an Editor's document, from a language server
// or any checker: From and To are positions in the document, in UTF-8
// bytes, as every position of an Editor.
type Diagnostic struct {
	From, To int
	Severity Severity
	Message  string
	// Source names what found the problem (a server, a linter) and Code
	// its name for it; the popup shows them under Message.
	Source, Code string
}

// diagnosticDot is the diameter of a diagnostic's dot in the gutter, and
// waveHeight the height of its wavy underline.
const (
	diagnosticDot = unit.Dp(6)
	waveHeight    = unit.Dp(3)
)

// SetDiagnostics replaces the editor's diagnostics with d, positions in its
// current document. From then on they follow edits as text does: a
// diagnostic whose whole range is deleted is dropped. A position outside the
// document is clamped to it, and one inside a character goes to its start.
// The editor draws them from its next Layout: a wavy underline in the
// Theme's Diagnostics color of each one's severity, a dot in the gutter of
// the line each starts on (the worst severity's), and the messages in a
// popup while the pointer rests over a range. Like Dispatch, it is called on
// the goroutine that lays the editor out.
func (e *Editor) SetDiagnostics(d []Diagnostic) {
	e.diags = make([]Diagnostic, len(d))
	for i, x := range d {
		x.From, x.To = e.clampPos(x.From), e.clampPos(x.To)
		if x.To < x.From {
			x.From, x.To = x.To, x.From
		}
		e.diags[i] = x
	}
	slices.SortStableFunc(e.diags, func(a, b Diagnostic) int { return cmp.Compare(a.From, b.From) })
	e.jumped = false
}

// Diagnostics are the editor's diagnostics, mapped through every edit since
// they were set, in document order. It is a copy.
func (e *Editor) Diagnostics() []Diagnostic { return slices.Clone(e.diags) }

func (e *Editor) clampPos(pos int) int {
	doc := e.state.doc
	pos = min(max(pos, 0), len(doc))
	for pos > 0 && pos < len(doc) && !utf8.RuneStart(doc[pos]) {
		pos--
	}
	return pos
}

// mapDiagnostics moves the diagnostics through c.
func (e *Editor) mapDiagnostics(c ChangeSet) {
	if c.Empty() || len(e.diags) == 0 {
		return
	}
	kept := e.diags[:0]
	for _, d := range e.diags {
		from, to := c.MapPos(d.From, 1), c.MapPos(d.To, -1)
		if d.From < d.To && from >= to {
			continue
		}
		d.From, d.To = from, max(to, from)
		kept = append(kept, d)
	}
	e.diags = kept
}

// NextDiagnostic moves the cursor to the start of the next diagnostic of
// severity after it, wrapping around to the first, and shows its message
// there until the selection moves; severity 0 takes any. It reports whether
// there was one. F8 runs it for any severity.
func (e *Editor) NextDiagnostic(severity Severity) bool {
	head := e.state.sel.Main().Head()
	return e.jumpToDiagnostic(severity, func(d Diagnostic) bool { return d.From > head }, false)
}

// PrevDiagnostic is NextDiagnostic backwards: the last diagnostic of
// severity before the cursor, wrapping around to the last. Shift+F8 runs it
// for any severity.
func (e *Editor) PrevDiagnostic(severity Severity) bool {
	head := e.state.sel.Main().Head()
	return e.jumpToDiagnostic(severity, func(d Diagnostic) bool { return d.From < head }, true)
}

// jumpToDiagnostic moves to the first diagnostic of severity past says is
// past the cursor, the first of them all when none is, searching backwards
// when back is set.
func (e *Editor) jumpToDiagnostic(severity Severity, past func(Diagnostic) bool, back bool) bool {
	var found []Diagnostic
	for _, d := range e.diags {
		if severity == 0 || d.severity() == severity {
			found = append(found, d)
		}
	}
	if len(found) == 0 {
		return false
	}
	if back {
		slices.Reverse(found)
	}
	to := found[0]
	if i := slices.IndexFunc(found, past); i >= 0 {
		to = found[i]
	}
	e.mustDispatch(TransactionSpec{Selection: SingleSelection(to.From, to.From), ScrollIntoView: true, UserEvent: "select"})
	e.jumped = true
	return true
}

// severity is d's, an unset one taken as an error.
func (d Diagnostic) severity() Severity {
	if d.Severity < SeverityError || d.Severity > SeverityHint {
		return SeverityError
	}
	return d.Severity
}

func (c DiagnosticColors) of(s Severity) color.NRGBA {
	switch s {
	case SeverityWarning:
		return c.Warning
	case SeverityInfo:
		return c.Info
	case SeverityHint:
		return c.Hint
	}
	return c.Error
}

// visibleLines are the first and last document lines on screen.
func (e *Editor) visibleLines() (first, last int) {
	v := e.view
	return v.LineAtHeight(v.scroll.Y), v.LineAtHeight(v.scroll.Y + e.f.size.Y)
}

// rangeBoxes are the boxes, a row high and one per row on screen, that the
// range from, to covers, in pixels from the editor's top left. An empty
// range covers the character after it, or before it at the end of a line.
func (e *Editor) rangeBoxes(from, to int) []image.Rectangle {
	v, f := e.view, e.f
	a, b := e.cellAt(from), e.cellAt(to)
	if a == b {
		if b.col < len(v.doc.lines[b.line]) {
			b.col++
		} else if a.col > 0 {
			a.col--
		}
	}
	first, last := e.visibleLines()
	var boxes []image.Rectangle
	for line := max(a.line, first); line <= min(b.line, last); line++ {
		lo, hi := 0, len(v.doc.lines[line])
		if line == a.line {
			lo = a.col
		}
		if line == b.line {
			hi = b.col
		}
		for seg := range v.wraps[line] {
			start, end := v.segmentSpan(line, seg)
			c0, c1 := max(lo, start), min(hi, end)
			if c0 >= c1 {
				continue
			}
			x := f.g.textX - v.scrollX() + (v.hang(line, seg)-start)*f.cw
			y := v.tops[line] + seg*f.ch - v.scroll.Y
			boxes = append(boxes, image.Rect(x+c0*f.cw, y, x+c1*f.cw, y+f.ch))
		}
	}
	return boxes
}

// paintDiagnostics draws the wavy underlines and the gutter's dots.
func (e *Editor) paintDiagnostics(gtx layout.Context) {
	if len(e.diags) == 0 {
		return
	}
	f := e.f
	code := clip.Rect(image.Rect(f.g.gutterW, 0, f.size.X, f.size.Y)).Push(gtx.Ops)
	for _, d := range e.diags {
		c := e.Theme.Diagnostics.of(d.severity())
		for _, b := range e.rangeBoxes(d.From, d.To) {
			paintWave(gtx, c, b.Min.X, b.Max.X, b.Max.Y)
		}
	}
	code.Pop()
	e.paintDiagnosticDots(gtx)
}

// paintWave draws a wavy line from x0 to x1 ending at bottom, the wave of
// @codemirror/lint's lintRange underline: 6px long, 3px high.
func paintWave(gtx layout.Context, c color.NRGBA, x0, x1, bottom int) {
	h := float32(gtx.Dp(waveHeight))
	step := 2 * h
	defer clip.Rect(image.Rect(x0, bottom-int(h)-1, x1, bottom)).Push(gtx.Ops).Pop()
	var p clip.Path
	p.Begin(gtx.Ops)
	base := float32(bottom) - h/6
	p.MoveTo(f32.Pt(float32(x0), base))
	for x := float32(x0); x < float32(x1); x += step {
		p.LineTo(f32.Pt(x+step/3, base-h*2/3))
		p.LineTo(f32.Pt(x+step/2, base-h*2/3))
		p.LineTo(f32.Pt(x+step*5/6, base))
		p.LineTo(f32.Pt(x+step, base))
	}
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(1))}.Op())
}

// paintDiagnosticDots draws a dot in the gutter's last cell, beside the
// line numbers, of each line on screen a diagnostic starts on, in the
// color of the worst of them.
func (e *Editor) paintDiagnosticDots(gtx layout.Context) {
	v, f := e.view, e.f
	first, last := e.visibleLines()
	worst := map[int]Severity{}
	for _, d := range e.diags {
		line := e.state.LineAt(d.From).Number - 1
		if line < first || line > last {
			continue
		}
		if s, ok := worst[line]; !ok || d.severity() < s {
			worst[line] = d.severity()
		}
	}
	size := gtx.Dp(diagnosticDot)
	for line, s := range worst {
		cx := f.g.gutterW - f.cw/2
		cy := v.tops[line] - v.scroll.Y + f.ch/2
		r := image.Rect(cx-size/2, cy-size/2, cx-size/2+size, cy-size/2+size)
		paint.FillShape(gtx.Ops, e.Theme.Diagnostics.of(s), clip.Ellipse(r).Op(gtx.Ops))
	}
}

// diagnosticsAt are the diagnostics whose underline the point at, in the
// editor, is over.
func (e *Editor) diagnosticsAt(at image.Point) []Diagnostic {
	var out []Diagnostic
	for _, d := range e.diags {
		for _, b := range e.rangeBoxes(d.From, d.To) {
			if at.In(b) {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

// diagnosticsAtCursor are the diagnostics the main cursor is on, the ones a
// jump to a diagnostic shows.
func (e *Editor) diagnosticsAtCursor() []Diagnostic {
	head := e.state.sel.Main().Head()
	var out []Diagnostic
	for _, d := range e.diags {
		if d.From <= head && head <= d.To {
			out = append(out, d)
		}
	}
	return out
}

// diagnosticBlocks are a popup's blocks for ds: each message, after a dot
// of its severity's color, and its source and code under it.
func (e *Editor) diagnosticBlocks(ds []Diagnostic) []mdBlock {
	var blocks []mdBlock
	for i, d := range ds {
		if i > 0 {
			blocks = append(blocks, mdBlock{kind: ruleBlock})
		}
		msg := plainBlocks(d.Message, false)
		msg[0].bullet, msg[0].bulletColor = "● ", e.Theme.Diagnostics.of(d.severity())
		blocks = append(blocks, msg...)
		if origin := diagnosticOrigin(d); origin != "" {
			blocks = append(blocks, plainBlocks(origin, true)...)
		}
	}
	return blocks
}

// diagnosticOrigin is d's source and code, as "source · code".
func diagnosticOrigin(d Diagnostic) string {
	switch {
	case d.Source != "" && d.Code != "":
		return d.Source + " · " + d.Code
	case d.Source != "":
		return d.Source
	}
	return d.Code
}
