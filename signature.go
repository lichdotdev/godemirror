package godemirror

import (
	"context"
	"image"
	"slices"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
)

// Signature is the signature of the call being typed, as SignatureHelp
// tells it: Label, code, with the ranges of its parameters in Params, in
// bytes of Label, Active the one being typed (-1 for none), and Doc,
// Markdown, under it.
type Signature struct {
	Label  string
	Params []SignatureParam
	Active int
	Doc    string
}

// SignatureParam is a parameter's range in a Signature's Label.
type SignatureParam struct {
	From, To int
}

// signatureState is the signature shown, or being asked for.
type signatureState struct {
	open bool
	sig  Signature
	call *call[signatureAnswer]
	// due is when typing paused long enough after a transaction to ask
	// again, zero when nothing waits.
	due time.Time
}

type signatureAnswer struct {
	sig Signature
	ok  bool
}

// askSignature asks SignatureHelp for the signature at the cursor, after
// the frame's updates are reported; trigger is the character typed that
// asked, empty when the signature is asked again.
func (e *Editor) askSignature(trigger string) {
	hook := e.SignatureHelp
	if hook == nil || len(e.state.sel.ranges) != 1 {
		return
	}
	g := &e.signature
	g.open, g.due = true, time.Time{}
	g.call.stop()
	g.call = nil
	s, pos := e.state, e.state.sel.Main().Head()
	e.later(func() {
		if !g.open || g.call != nil {
			return
		}
		g.call = startCall(func(ctx context.Context) signatureAnswer {
			sig, ok := hook(ctx, s, pos, trigger)
			return signatureAnswer{sig, ok}
		})
	})
}

func (e *Editor) closeSignature() {
	e.signature.call.stop()
	e.signature = signatureState{}
}

// signatureTyped asks for the signature after a trigger character, or
// again while one shows.
func (e *Editor) signatureTyped(text string) {
	if last := []rune(text); len(last) > 0 && slices.Contains(e.SignatureTriggers, string(last[len(last)-1])) {
		e.askSignature(string(last[len(last)-1]))
	}
}

// takeSignature takes SignatureHelp's answer when it comes, closing the
// popup when it has none, and asks again once typing paused after a
// transaction, the signature shown staying until the answer.
func (e *Editor) takeSignature(gtx layout.Context) {
	g := &e.signature
	if !g.open {
		return
	}
	if g.call != nil {
		if a, ok := g.call.poll(gtx); ok {
			g.call = nil
			if !a.ok {
				e.closeSignature()
				return
			}
			g.sig = a.sig
		}
	}
	switch {
	case g.due.IsZero():
	case gtx.Now.Before(g.due):
		gtx.Execute(op.InvalidateCmd{At: g.due})
	default:
		e.askSignature("")
	}
}

// signatureKey closes the signature on Escape, and reports whether it did.
func (e *Editor) signatureKey(ev key.Event) bool {
	if !e.signature.open || ev.Name != key.NameEscape || ev.Modifiers != 0 {
		return false
	}
	e.closeSignature()
	return true
}

// paintSignature draws the signature above the cursor, its active
// parameter bold.
func (e *Editor) paintSignature(gtx layout.Context) {
	g := &e.signature
	if !g.open || g.sig.Label == "" {
		return
	}
	blocks := []mdBlock{{inlines: signatureInlines(g.sig)}}
	if g.sig.Doc != "" {
		blocks = append(blocks, parseMarkdown(g.sig.Doc)...)
	}
	main := e.state.sel.Main()
	x, top := e.coordsAt(main.Head(), main.Assoc())
	x += e.f.g.textX - e.view.scrollX()
	anchor := image.Rect(x, top-e.view.scroll.Y, x+1, top-e.view.scroll.Y+e.f.ch)
	e.proseBox(gtx, signaturePopup, blocks, e.maxProseWidth(gtx), func(size image.Point) image.Rectangle {
		return e.placePopup(gtx, anchor, size, true)
	})
}

// signatureInlines are sig's label as code, its active parameter bold.
func signatureInlines(sig Signature) []inline {
	label := sig.Label
	if sig.Active < 0 || sig.Active >= len(sig.Params) {
		return []inline{{text: label, code: true}}
	}
	p := sig.Params[sig.Active]
	from, to := min(max(p.From, 0), len(label)), min(max(p.To, 0), len(label))
	from = min(from, to)
	return []inline{
		{text: label[:from], code: true},
		{text: label[from:to], code: true, bold: true},
		{text: label[to:], code: true},
	}
}
