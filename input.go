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

// selector is what a press, and a drag after it, on the view select.
type selector interface {
	// press is the clicks-th press in a row at at, with mods held.
	press(f frame, at image.Point, clicks int, mods key.Modifiers)
	// drag is the pointer, still held, moved to at.
	drag(f frame, at image.Point)
}

// viewSelector selects in the view's own selection.
type viewSelector struct{ v *View }

func (s viewSelector) press(f frame, at image.Point, clicks int, mods key.Modifiers) {
	s.v.selectPress(s.v.hit(f, at), clicks, mods.Contain(key.ModShift))
}

func (s viewSelector) drag(f frame, at image.Point) { s.v.sel.head = s.v.hit(f, at) }

// handlePointer scrolls the view and selects with the primary button. It
// reports whether a press started a new selection, which a split uses to
// clear the other side's.
func (v *View) handlePointer(gtx layout.Context, f frame) (pressed bool) {
	pressed = v.takePointer(gtx, f)
	limit := v.scrollLimit(gtx, f)
	v.scroll.X = min(max(v.scroll.X, 0), limit.X)
	v.scroll.Y = min(max(v.scroll.Y, 0), limit.Y)
	v.autoScroll(gtx, f, limit)
	v.hot = v.decoration(v.hover).HoverGroup
	return pressed
}

// scrollLimit is how far the view can scroll down and sideways.
func (v *View) scrollLimit(gtx layout.Context, f frame) image.Point {
	limit := image.Pt(0, max(v.tops[len(v.tops)-1]-f.size.Y, 0))
	if !v.Wrap {
		limit.X = max(v.longest*f.cw+gtx.Dp(scrollEndMargin)-(f.size.X-f.g.textX), 0)
	}
	return limit
}

// takePointer takes the pointer events addressed to the view, and tells Gio
// how far a wheel may scroll it from where it is now.
func (v *View) takePointer(gtx layout.Context, f frame) (pressed bool) {
	limit := v.scrollLimit(gtx, f)
	sideways := pointer.ScrollRange{Min: -v.scroll.X, Max: limit.X - v.scroll.X}
	down := pointer.ScrollRange{Min: -v.scroll.Y, Max: limit.Y - v.scroll.Y}
	// Shift turns a vertical wheel sideways, which Gio leaves to the handler:
	// it clamps the wheel by the vertical range first, so that range also
	// admits what the sideways one would. A nested view does so only while
	// Shift is held, or it would take every wheel its parent should scroll by.
	vertical := down
	if !v.Nested || v.mods.Contain(key.ModShift) {
		vertical = pointer.ScrollRange{Min: min(down.Min, sideways.Min), Max: max(down.Max, sideways.Max)}
	}
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  v,
			Kinds:   pointer.Scroll | pointer.Press | pointer.Drag | pointer.Release | pointer.Move | pointer.Enter | pointer.Leave,
			ScrollX: sideways,
			ScrollY: vertical,
		})
		if !ok {
			return pressed
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		if v.pointerEvent(gtx, f, e) {
			pressed = true
		}
	}
}

func (v *View) pointerEvent(gtx layout.Context, f frame, e pointer.Event) (pressed bool) {
	at := e.Position.Round()
	v.hover = v.lineUnder(f, at)
	v.mods = e.Modifiers
	v.pointerAt, v.pointerIn = at, true
	switch e.Kind {
	case pointer.Leave:
		v.hover = -1
		v.pointerIn = false
	case pointer.Scroll:
		delta := image.Pt(int(e.Scroll.X), int(e.Scroll.Y))
		if e.Modifiers.Contain(key.ModShift) && delta.X == 0 {
			delta = image.Pt(delta.Y, 0)
		}
		if delta != (image.Point{}) {
			v.scroll, v.glide, v.userScrolled = v.scroll.Add(delta), glide{}, true
		}
	case pointer.Press:
		if !e.Buttons.Contain(pointer.ButtonPrimary) || at.X < f.g.gutterW || v.onWidget(at) {
			return false
		}
		gtx.Execute(key.FocusCmd{Tag: v})
		v.selecting.press(f, at, v.press.next(gtx.Now, at), e.Modifiers)
		v.press.dragging = true
		v.drag.pointer = at
		return true
	case pointer.Drag:
		if v.press.dragging {
			v.selecting.drag(f, at)
			v.drag.pointer = at
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
