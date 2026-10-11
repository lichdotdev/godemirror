package godemirror

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// The speed a held selection scrolls at is autoScrollMinSpeed plus
// autoScrollGain for every pixel the pointer is beyond the edge, capped at
// autoScrollMaxSpeed. The numbers are a judgement call made by feel, not
// CodeMirror's: about six rows a second at the edge, a page a second a hand's
// width away, and a ceiling that still lets the eye follow.
const (
	// autoScrollMinSpeed and autoScrollMaxSpeed are in dp per second.
	autoScrollMinSpeed = unit.Dp(90)
	autoScrollMaxSpeed = unit.Dp(2400)
	// autoScrollGain is dp per second for each dp beyond the edge, so a
	// pure rate: 1/s.
	autoScrollGain = 12
	// autoScrollMaxStep bounds the time one step covers, so a frame that came
	// late does not throw the view a page.
	autoScrollMaxStep = 100 * time.Millisecond
)

// dragScroll is the state of a selection held beyond an edge of the view.
type dragScroll struct {
	// pointer is where the pointer was last seen while dragging.
	pointer image.Point
	// at is when the view last scrolled, zero while the pointer is inside.
	at time.Time
	// rest is the part of a pixel not scrolled yet.
	rest f32.Point
}

// beyond is how far p is past the code area: negative above or left of it,
// positive below or right of it, 0 inside. Sideways counts only when the view
// is not wrapped.
func (v *View) beyond(f frame, p image.Point) image.Point {
	var out image.Point
	switch {
	case p.Y < 0:
		out.Y = p.Y
	case p.Y > f.size.Y:
		out.Y = p.Y - f.size.Y
	}
	if v.Wrap {
		return out
	}
	switch {
	case p.X < f.g.gutterW:
		out.X = p.X - f.g.gutterW
	case p.X > f.size.X:
		out.X = p.X - f.size.X
	}
	return out
}

// autoScrollSpeed is the signed speed in px per second of a pointer beyond an
// edge by over px, 0 for one that is not beyond it.
func autoScrollSpeed(gtx layout.Context, over int) float32 {
	if over == 0 {
		return 0
	}
	speed := min(
		float32(gtx.Dp(autoScrollMinSpeed))+float32(abs(over))*autoScrollGain,
		float32(gtx.Dp(autoScrollMaxSpeed)),
	)
	if over < 0 {
		return -speed
	}
	return speed
}

// autoScroll scrolls the view for the time since its last step while a
// selection is dragged beyond an edge, moves the selection's end to what is
// under the pointer now, and asks for the next frame while there is room left
// to scroll. limit is how far the view can scroll.
func (v *View) autoScroll(gtx layout.Context, f frame, limit image.Point) {
	d := &v.drag
	over := v.beyond(f, d.pointer)
	if !v.press.dragging || over == (image.Point{}) {
		*d = dragScroll{pointer: d.pointer}
		return
	}
	if d.at.IsZero() {
		d.at = gtx.Now
	}
	elapsed := float32(min(gtx.Now.Sub(d.at), autoScrollMaxStep).Seconds())
	d.at = gtx.Now
	v.scroll.X, d.rest.X = advance(v.scroll.X, d.rest.X+autoScrollSpeed(gtx, over.X)*elapsed, limit.X)
	v.scroll.Y, d.rest.Y = advance(v.scroll.Y, d.rest.Y+autoScrollSpeed(gtx, over.Y)*elapsed, limit.Y)
	v.sel.head = v.hit(f, d.pointer)
	if room(over.X, v.scroll.X, limit.X) || room(over.Y, v.scroll.Y, limit.Y) {
		gtx.Execute(op.InvalidateCmd{})
	}
}

// advance moves pos by the whole pixels of by, within [0, limit], and returns
// the fraction of a pixel left over. An end of the range keeps no fraction, so
// a pointer held there does not store up a jump for the way back.
func advance(pos int, by float32, limit int) (int, float32) {
	whole := int(by)
	next := min(max(pos+whole, 0), limit)
	if next != pos+whole {
		return next, 0
	}
	return next, by - float32(whole)
}

// room reports whether pos can still move the way over points, 0 meaning
// nowhere.
func room(over, pos, limit int) bool {
	return over < 0 && pos > 0 || over > 0 && pos < limit
}
