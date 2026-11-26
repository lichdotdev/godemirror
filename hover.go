package godemirror

import (
	"context"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/tags"
)

// hoverDelay is how long the pointer rests over the text before the editor
// shows what is under it.
const hoverDelay = 500 * time.Millisecond

// DefaultDefinitionHint is the hint a new Editor shows under a hover while
// it has a Definition.
const DefaultDefinitionHint = "Ctrl+click to go to definition"

// HoverInfo is what an Editor's Hover shows over a name: Code, drawn as
// code, a signature say, over Text, the documentation, both plain text.
type HoverInfo struct {
	Code, Text string
}

// hoverState is what the pointer rests over.
type hoverState struct {
	at    image.Point
	since time.Time
	// fired is whether it rested long enough, and then name is the name
	// under it, diags the diagnostics, info what Hover answered.
	fired bool
	name  SelectionRange
	diags []Diagnostic
	info  HoverInfo
	call  *call[HoverInfo]
}

// trackHover follows the pointer resting over the text: after hoverDelay in
// one place it takes the diagnostics and the name under it, and asks Hover
// about the name; it keeps them while the pointer stays over them.
func (e *Editor) trackHover(gtx layout.Context) {
	v, h := e.view, &e.hover
	if !v.pointerIn || v.press.dragging || v.pointerAt.X < e.f.g.gutterW {
		e.clearHover()
		return
	}
	if v.pointerAt != h.at {
		if !h.fired || !e.stillOver(v.pointerAt) {
			e.clearHover()
			h.since = gtx.Now
		}
		h.at = v.pointerAt
	}
	if !h.fired {
		if wait := h.since.Add(hoverDelay); gtx.Now.Before(wait) {
			gtx.Execute(op.InvalidateCmd{At: wait})
			return
		}
		e.fireHover()
	}
	if h.call != nil {
		if info, ok := h.call.poll(gtx); ok {
			h.info, h.call = info, nil
		}
	}
}

func (e *Editor) fireHover() {
	h := &e.hover
	h.fired = true
	h.diags = e.diagnosticsAt(h.at)
	name, ok := e.nameAt(h.at)
	if !ok {
		return
	}
	h.name = name
	if hook := e.Hover; hook != nil {
		s, pos := e.state, name.from
		e.later(func() {
			if !h.fired || h.name != name || h.call != nil {
				return
			}
			h.call = startCall(func(ctx context.Context) HoverInfo { return hook(ctx, s, pos) })
		})
	}
}

// stillOver reports whether at is over what the pointer rested on.
func (e *Editor) stillOver(at image.Point) bool {
	h := &e.hover
	if h.name.from < h.name.to && e.over(at, h.name.from, h.name.to) {
		return true
	}
	for _, d := range h.diags {
		if e.over(at, d.From, d.To) {
			return true
		}
	}
	return false
}

func (e *Editor) over(at image.Point, from, to int) bool {
	for _, b := range e.rangeBoxes(from, to) {
		if at.In(b) {
			return true
		}
	}
	return false
}

func (e *Editor) clearHover() {
	e.hover.call.stop()
	e.hover = hoverState{at: e.hover.at, since: e.hover.since}
}

// nameAt is the word under at, a point in the editor, ok false when there
// is none.
func (e *Editor) nameAt(at image.Point) (SelectionRange, bool) {
	pos, assoc := e.posAndSide(e.f, at)
	r := e.state.groupAt(pos, assoc)
	if r.from == r.to || categorize(e.state.doc[r.from:r.to]) != categoryWord || !e.over(at, r.from, r.to) {
		return SelectionRange{}, false
	}
	return r, true
}

// paintHover draws the popup of what the pointer rests over, or of the
// diagnostics at the cursor after a jump to one.
func (e *Editor) paintHover(gtx layout.Context) {
	h, cols := &e.hover, e.popupCols(gtx)
	var rows []popupRow
	var anchor image.Rectangle
	above := false
	switch {
	case h.fired && (len(h.diags) > 0 || h.info != HoverInfo{}):
		rows = e.diagnosticRows(h.diags, cols)
		if info := e.hoverRows(h.info, cols); len(info) > 0 {
			if len(rows) > 0 {
				rows = append(rows, popupRow{rule: true})
			}
			rows, above = append(rows, info...), true
		}
		anchor = e.hoverAnchor()
	case e.jumped:
		rows = e.diagnosticRows(e.diagnosticsAtCursor(), cols)
		main := e.state.sel.Main()
		x, top := e.coordsAt(main.Head(), main.Assoc())
		x += e.f.g.textX - e.view.scrollX()
		anchor = image.Rect(x, top-e.view.scroll.Y, x+1, top-e.view.scroll.Y+e.f.ch)
	}
	if len(rows) == 0 {
		return
	}
	var footer []popupRow
	if above && e.Definition != nil && e.DefinitionHint != "" {
		footer = []popupRow{{spans: []textSpan{{text: e.DefinitionHint, color: e.Theme.Popup.Muted}}}}
	}
	rows, r := e.fitPopup(gtx, rows, footer, anchor, above)
	e.paintTextPopup(gtx, rows, r)
}

// hoverAnchor is what the hover popup points at: the name under the
// pointer, else the first diagnostic under it, else the pointer.
func (e *Editor) hoverAnchor() image.Rectangle {
	h := &e.hover
	if h.name.from < h.name.to {
		if boxes := e.rangeBoxes(h.name.from, h.name.to); len(boxes) > 0 {
			return boxes[0]
		}
	}
	if len(h.diags) > 0 {
		if boxes := e.rangeBoxes(h.diags[0].From, h.diags[0].To); len(boxes) > 0 {
			return boxes[0]
		}
	}
	return image.Rectangle{Min: h.at, Max: h.at.Add(image.Pt(1, 1))}
}

// hoverRows are a popup's rows for info: its code, a rule, and its text.
func (e *Editor) hoverRows(info HoverInfo, cols int) []popupRow {
	p := e.Theme.Popup
	var rows []popupRow
	code := e.Theme.tagColor(tags.Function(tags.VariableName))
	for l := range strings.SplitSeq(strings.TrimSpace(info.Code), "\n") {
		if l != "" {
			rows = append(rows, popupRow{spans: []textSpan{{text: expandTabs(l), color: code}}})
		}
	}
	if text := strings.TrimSpace(info.Text); text != "" {
		if len(rows) > 0 {
			rows = append(rows, popupRow{rule: true})
		}
		rows = append(rows, textRows(text, cols, p.Foreground)...)
	}
	return rows
}

// expandTabs is l with each tab as four spaces, a popup having no tab
// stops.
func expandTabs(l string) string { return strings.ReplaceAll(l, "\t", "    ") }

// tagColor is the color t's highlighting gives code tagged tag.
func (t Theme) tagColor(tag *tags.Tag) color.NRGBA {
	return t.Highlight.style(&lezer.TagSet{Tags: []*tags.Tag{tag}}, t.Foreground).Color
}

// linking reports whether the name under the pointer shows as a link: a
// Definition is set and Ctrl, the shortcut modifier, is held.
func (e *Editor) linking() bool {
	return e.Definition != nil && e.view.pointerIn && e.shortcutDown
}

// trackLink takes the name under the pointer as a link while linking, and
// points the cursor at it.
func (e *Editor) trackLink() {
	e.link = SelectionRange{}
	if !e.linking() {
		return
	}
	if name, ok := e.nameAt(e.view.pointerAt); ok {
		e.link = name
		e.view.cursor = pointer.CursorPointer
	}
}

// paintLink underlines the link.
func (e *Editor) paintLink(gtx layout.Context) {
	if e.link.from == e.link.to {
		return
	}
	thick := max(gtx.Dp(1), 1)
	for _, b := range e.rangeBoxes(e.link.from, e.link.to) {
		line := image.Rect(b.Min.X, b.Max.Y-thick-1, b.Max.X, b.Max.Y-1)
		paint.FillShape(gtx.Ops, e.Theme.Foreground, clip.Rect(line).Op())
	}
}

// onShortcutKey follows Ctrl through the keys the editor takes, as
// onAltKey follows Alt.
func (e *Editor) onShortcutKey(ev key.Event) {
	isShortcut := ev.Name == key.NameCtrl || ev.Name == key.NameCommand
	switch {
	case ev.State == key.Press:
		e.shortcutDown = isShortcut || ev.Modifiers.Contain(key.ModShortcut)
	case isShortcut || !ev.Modifiers.Contain(key.ModShortcut):
		e.shortcutDown = false
	}
}
