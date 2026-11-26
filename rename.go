package godemirror

import (
	"context"
	"image"
	"unicode/utf8"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// TextEdit replaces the span Start, End of a document with Text.
type TextEdit struct {
	Start, End LineColumn
	Text       string
}

// DocumentEdit are edits to one document, as a rename or a code action
// makes them: URI names the document, empty for the editor's own; the
// edits' spans are all in the document before them.
type DocumentEdit struct {
	URI   string
	Edits []TextEdit
}

// renaming is the name being typed for a rename, in the input at the
// symbol: its range in the document, the text typed, and whether the text
// is still the old name, all of it selected.
type renaming struct {
	from, to int
	text     string
	selected bool
}

// renameState is a rename: the name being typed, the call applying it, and
// the error it met, shown at the symbol until the next transaction.
type renameState struct {
	input *renaming
	call  *call[renameAnswer]
	err   string
	at    int
}

type renameAnswer struct {
	edits []DocumentEdit
	err   error
	doc   string
}

// startRename opens the input at the name at the cursor, holding the name.
func (e *Editor) startRename() {
	if e.Rename == nil {
		return
	}
	r, ok := e.state.wordAt(e.state.sel.Main().Head())
	if !ok {
		return
	}
	e.closeCompletion()
	e.rename = renameState{input: &renaming{from: r.from, to: r.to, text: e.state.doc[r.from:r.to], selected: true}}
}

// renameKey runs a key on the input while it is open, and reports whether
// it took it: every key goes to the input then.
func (e *Editor) renameKey(ev key.Event) bool {
	in := e.rename.input
	if in == nil {
		return false
	}
	switch ev.Name {
	case key.NameReturn, key.NameEnter:
		e.applyRename()
	case key.NameEscape:
		e.rename.input = nil
	case key.NameDeleteBackward:
		if in.selected {
			in.text, in.selected = "", false
		} else if _, n := utf8.DecodeLastRuneInString(in.text); n > 0 {
			in.text = in.text[:len(in.text)-n]
		}
	}
	return true
}

// renameTyped takes typed text into the input while it is open, and
// reports whether it did.
func (e *Editor) renameTyped(text string) bool {
	in := e.rename.input
	if in == nil {
		return false
	}
	if in.selected {
		in.text, in.selected = "", false
	}
	in.text += text
	return true
}

// applyRename asks Rename for the edits renaming the symbol to the name
// typed; a name unchanged or empty renames nothing.
func (e *Editor) applyRename() {
	in, hook := e.rename.input, e.Rename
	e.rename.input = nil
	old := e.state.doc[in.from:in.to]
	if in.text == "" || in.text == old || hook == nil {
		return
	}
	s, pos, name := e.state, in.from, in.text
	e.rename.at = in.from
	e.later(func() {
		e.rename.call.stop()
		e.rename.call = startCall(func(ctx context.Context) renameAnswer {
			edits, err := hook(ctx, s, pos, name)
			return renameAnswer{edits, err, s.doc}
		})
	})
}

// takeRename applies Rename's answer when it comes: the edits to the
// document as one transaction, when it has not changed since, and the
// others to OnWorkspaceEdit after the frame. An error shows at the symbol.
func (e *Editor) takeRename(gtx layout.Context) {
	if e.rename.call == nil {
		return
	}
	a, ok := e.rename.call.poll(gtx)
	if !ok {
		return
	}
	e.rename.call = nil
	if a.err != nil {
		e.rename.err = a.err.Error()
		return
	}
	if a.doc == e.state.doc {
		e.applyEdits(a.edits, "input.rename")
	}
}

// applyEdits makes the edits to the editor's own document one transaction,
// and keeps those to others for OnWorkspaceEdit after the frame.
func (e *Editor) applyEdits(edits []DocumentEdit, userEvent string) {
	var changes []ChangeSpec
	for _, d := range edits {
		if d.URI != "" {
			e.workspaceEdits = append(e.workspaceEdits, d)
			continue
		}
		for _, t := range d.Edits {
			changes = append(changes, ChangeSpec{From: e.posOfLineColumn(t.Start), To: e.posOfLineColumn(t.End), Insert: t.Text})
		}
	}
	if len(changes) > 0 {
		e.mustDispatch(TransactionSpec{Changes: changes, UserEvent: userEvent})
	}
}

// reportWorkspaceEdits hands the edits to other documents to
// OnWorkspaceEdit, after the frame.
func (e *Editor) reportWorkspaceEdits() {
	edits := e.workspaceEdits
	e.workspaceEdits = nil
	if len(edits) > 0 && e.OnWorkspaceEdit != nil {
		e.OnWorkspaceEdit(edits)
	}
}

// paintRename draws the input over the symbol being renamed, or the error
// the rename met.
func (e *Editor) paintRename(gtx layout.Context) {
	if msg := e.rename.err; msg != "" {
		e.proseBox(gtx, renamePopup, plainBlocks(msg, false), e.maxProseWidth(gtx), func(size image.Point) image.Rectangle {
			return e.placePopup(gtx, e.posBox(e.rename.at), size, false)
		})
	}
	in := e.rename.input
	if in == nil {
		return
	}
	at := e.posBox(in.from)
	cells := max(len(textCells(in.text))+1, renameMinCells)
	pad := gtx.Dp(popupPadY)
	r := image.Rect(at.Min.X-pad, at.Min.Y-pad, at.Min.X+cells*e.f.cw+pad, at.Max.Y+pad)
	e.paintPopupBox(gtx, r)
	p := e.Theme.Popup
	textAt := image.Pt(at.Min.X, at.Min.Y)
	if in.selected {
		sel := image.Rect(textAt.X, textAt.Y, textAt.X+len(textCells(in.text))*e.f.cw, textAt.Y+e.f.ch)
		paint.FillShape(gtx.Ops, over(e.Theme.Selection, p.Background), clip.Rect(sel).Op())
	}
	e.view.paintTextSpans(gtx, []textSpan{{text: in.text, color: p.Foreground}}, textAt, r)
	x := textAt.X + len(textCells(in.text))*e.f.cw
	caret := image.Rect(x, textAt.Y, x+max(gtx.Dp(cursorWidth), 1), textAt.Y+e.f.ch)
	paint.FillShape(gtx.Ops, e.Theme.Cursor, clip.Rect(caret).Op())
}

// renameMinCells is how many cells wide the rename input is at least.
const renameMinCells = 16

// posBox is the box, a row high and no wider, where pos shows.
func (e *Editor) posBox(pos int) image.Rectangle {
	x, top := e.coordsAt(pos, 1)
	x += e.f.g.textX - e.view.scrollX()
	y := top - e.view.scroll.Y
	return image.Rect(x, y, x+1, y+e.f.ch)
}
