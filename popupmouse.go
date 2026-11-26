package godemirror

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// The popups an Editor draws, each with a pointer handler of its own over
// the box it was last drawn in, above the text: the pointer over one does not
// reach the code beneath.
const (
	menuPopup = iota
	docPopup
	hoverPopup
	signaturePopup
	renamePopup
	popupKinds
)

// popupScrollRange is how far one wheel event may scroll a popup, which
// clamps it to its own content.
const popupScrollRange = 1 << 20

// popupPointer is the popups' pointer handling.
type popupPointer struct {
	// tags are the popups' handlers; a byte each, so no two share an
	// address.
	tags [popupKinds]byte
	// boxes are where the popups were drawn in the last frame, empty for
	// one not drawn.
	boxes [popupKinds]image.Rectangle
	// over is the popup the pointer is over, -1 for none, and scroll how
	// far down each prose popup is scrolled, in pixels.
	over   int
	scroll [popupKinds]int
}

// popupHandler takes the pointer over r, where popup kind is drawn.
func (e *Editor) popupHandler(gtx layout.Context, kind int, r image.Rectangle) {
	p := &e.popups
	p.boxes[kind] = r
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &p.tags[kind])
	pointer.CursorDefault.Add(gtx.Ops)
}

// handlePopupPointer takes the pointer events over the popups: a press on
// a completion takes it, the wheel scrolls the popup under it, and the
// pointer over a popup keeps it open.
func (e *Editor) handlePopupPointer(gtx layout.Context) {
	p := &e.popups
	for kind := range p.tags {
		for {
			ev, ok := gtx.Event(pointer.Filter{
				Target:  &p.tags[kind],
				Kinds:   pointer.Press | pointer.Scroll | pointer.Enter | pointer.Leave | pointer.Move,
				ScrollY: pointer.ScrollRange{Min: -popupScrollRange, Max: popupScrollRange},
			})
			if !ok {
				break
			}
			if pe, ok := ev.(pointer.Event); ok {
				e.popupEvent(kind, pe)
			}
		}
	}
}

func (e *Editor) popupEvent(kind int, ev pointer.Event) {
	p := &e.popups
	switch ev.Kind {
	case pointer.Enter, pointer.Move:
		p.over = kind
	case pointer.Leave:
		if p.over == kind {
			p.over = -1
		}
	case pointer.Press:
		if kind == menuPopup && ev.Buttons.Contain(pointer.ButtonPrimary) {
			e.pressMenu(ev.Position.Round())
		}
	case pointer.Scroll:
		e.scrollPopup(kind, int(ev.Scroll.Y))
	}
}

// pressMenu takes the completion at at, a point in the menu.
func (e *Editor) pressMenu(at image.Point) {
	c := &e.completion
	box := e.popups.boxes[menuPopup]
	if !c.open || len(c.shown) == 0 || e.menuRowH == 0 {
		return
	}
	i := c.top + (at.Y-box.Min.Y-1)/e.menuRowH
	if i < c.top || i >= len(c.shown) {
		return
	}
	c.sel = i
	e.acceptCompletion()
}

// scrollPopup scrolls popup kind by dy pixels, the menu a row at least;
// drawing the popup keeps it within its content.
func (e *Editor) scrollPopup(kind, dy int) {
	if kind != menuPopup {
		e.popups.scroll[kind] += dy
		return
	}
	if dy == 0 {
		return
	}
	c := &e.completion
	rows := dy / max(e.menuRowH, 1)
	if rows == 0 {
		rows = 1
		if dy < 0 {
			rows = -1
		}
	}
	c.top = min(max(c.top+rows, 0), max(len(c.shown)-completionRows, 0))
}
