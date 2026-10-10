package godemirror

import (
	"image"
	"io"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/unit"
)

const (
	// multiClickWindow and multiClickSlop decide when a press continues a
	// double or triple click: GTK's default double-click time and distance.
	multiClickWindow = 400 * time.Millisecond
	multiClickSlop   = 5
)

// scrollEndMargin is the room left after the longest line once the view is
// scrolled all the way right, so its last character does not sit on the
// edge: twice the 0.5rem padding of CodeMirror's lines.
const scrollEndMargin = unit.Dp(16)

// press remembers the last primary press, to count clicks in a row.
type press struct {
	at       time.Time
	where    image.Point
	clicks   int
	dragging bool
}

func (p *press) next(now time.Time, where image.Point) int {
	d := where.Sub(p.where)
	near := max(abs(d.X), abs(d.Y)) <= multiClickSlop
	if near && now.Sub(p.at) <= multiClickWindow {
		p.clicks++
	} else {
		p.clicks = 1
	}
	p.at, p.where = now, where
	return p.clicks
}

func abs(n int) int { return max(n, -n) }

// handlePointer scrolls the view and selects with the primary button. It
// reports whether a press started a new selection, which a split uses to
// clear the other side's.
func (v *View) handlePointer(gtx layout.Context, f frame) (pressed bool) {
	maxY := max(v.tops[len(v.tops)-1]-f.size.Y, 0)
	maxX := 0
	if !v.Wrap {
		maxX = max(v.longest*f.cw+gtx.Dp(scrollEndMargin)-(f.size.X-f.g.textX), 0)
	}
	sideways := pointer.ScrollRange{Min: -v.scroll.X, Max: maxX - v.scroll.X}
	down := pointer.ScrollRange{Min: -v.scroll.Y, Max: maxY - v.scroll.Y}
	for {
		// Shift turns a vertical wheel sideways, which Gio leaves to the
		// handler: it clamps the wheel by the vertical range first, so that
		// range also admits what the sideways one would.
		ev, ok := gtx.Event(pointer.Filter{
			Target:  v,
			Kinds:   pointer.Scroll | pointer.Press | pointer.Drag | pointer.Release | pointer.Move | pointer.Enter | pointer.Leave,
			ScrollX: sideways,
			ScrollY: pointer.ScrollRange{Min: min(down.Min, sideways.Min), Max: max(down.Max, sideways.Max)},
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		if v.pointerEvent(gtx, f, e) {
			pressed = true
		}
	}
	v.scroll.X = min(max(v.scroll.X, 0), maxX)
	v.scroll.Y = min(max(v.scroll.Y, 0), maxY)
	v.hot = v.decoration(v.hover).HoverGroup
	return pressed
}

func (v *View) pointerEvent(gtx layout.Context, f frame, e pointer.Event) (pressed bool) {
	at := e.Position.Round()
	v.hover = v.lineUnder(f, at)
	switch e.Kind {
	case pointer.Leave:
		v.hover = -1
	case pointer.Scroll:
		delta := image.Pt(int(e.Scroll.X), int(e.Scroll.Y))
		if e.Modifiers.Contain(key.ModShift) && delta.X == 0 {
			delta = image.Pt(delta.Y, 0)
		}
		v.scroll = v.scroll.Add(delta)
	case pointer.Press:
		if !e.Buttons.Contain(pointer.ButtonPrimary) || at.X < f.g.gutterW || v.onWidget(at) {
			return false
		}
		gtx.Execute(key.FocusCmd{Tag: v})
		v.selectPress(v.hit(f, at), v.press.next(gtx.Now, at), e.Modifiers.Contain(key.ModShift))
		v.press.dragging = true
		return true
	case pointer.Drag:
		if v.press.dragging {
			v.sel.head = v.hit(f, at)
		}
	case pointer.Release, pointer.Cancel:
		v.press.dragging = false
	}
	return false
}

// Shortcut keys the view answers while it has focus.
const (
	keyCopy      = "C"
	keySelectAll = "A"
)

func (v *View) handleKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(
			// Gio only hands focus to a tag that asks for focus events, so the
			// FocusCmd a press sends needs this filter to land.
			key.FocusFilter{Target: v},
			key.Filter{Focus: v, Name: keyCopy, Required: key.ModShortcut},
			key.Filter{Focus: v, Name: keySelectAll, Required: key.ModShortcut},
		)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case keyCopy:
			if text := v.SelectedText(); text != "" {
				gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
			}
		case keySelectAll:
			last := len(v.doc.lines) - 1
			v.sel = selection{pos{}, pos{last, len(v.doc.lines[last])}}
		}
	}
}

// lineUnder is the document line on the row under p, -1 off the document or
// over a BlockWidget.
func (v *View) lineUnder(f frame, p image.Point) int {
	line, segment, ok := locate(v.tops, v.starts, f.ch, v.scroll.Y+p.Y)
	if !ok || segment < 0 {
		return -1
	}
	return line
}
