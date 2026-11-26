package godemirror

import (
	"math"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

const (
	// glideDuration is how long a jump the program makes slides for, eased
	// out: a judgement call by feel, long enough for the eye to follow where
	// the text went and short enough not to keep it waiting.
	glideDuration = 140 * time.Millisecond
	// glideMinRows is the farthest a jump goes in rows and still snaps, so
	// that typing or an arrow at an edge scrolls at once.
	glideMinRows = 3
)

// glide is a vertical scroll sliding to a target the program set.
type glide struct {
	// on is whether the view is sliding, or about to, to to; started
	// whether it has begun, from from at at.
	on, started bool
	from, to    int
	at          time.Time
}

// heading is where the view rests once its glide, if any, is over.
func (v *View) heading() int {
	if v.glide.on {
		return v.glide.to
	}
	return v.scroll.Y
}

// glideTo sets the view sliding to y from wherever it is when the next step
// runs.
func (v *View) glideTo(y int) {
	if y != v.heading() {
		v.glide = glide{on: true, to: y}
	}
}

// glideStep moves the view along its glide for gtx.Now, within the document,
// and asks for the next frame until it lands. A jump of glideMinRows rows or
// less, any while ReduceMotion is set, and any laid out without a clock
// (a zero gtx.Now), lands at once.
func (v *View) glideStep(gtx layout.Context, f frame) {
	g := &v.glide
	if !g.on {
		return
	}
	to := min(max(g.to, 0), max(v.tops[len(v.tops)-1]-f.size.Y, 0))
	if !g.started {
		if v.ReduceMotion || gtx.Now.IsZero() || abs(to-v.scroll.Y) <= glideMinRows*f.ch {
			v.scroll.Y, *g = to, glide{}
			return
		}
		g.started, g.from, g.at = true, v.scroll.Y, gtx.Now
	}
	t := float64(gtx.Now.Sub(g.at)) / float64(glideDuration)
	if t >= 1 || t < 0 {
		v.scroll.Y, *g = to, glide{}
		return
	}
	v.scroll.Y = g.from + int(math.Round(float64(to-g.from)*easeOut(t)))
	gtx.Execute(op.InvalidateCmd{})
}

// stepGlide moves the view along its glide, and then tells Gio how far a
// wheel may scroll it from there: the range handlePointer gave it is from
// where the view was before the step, and would stop a wheel short.
func (v *View) stepGlide(gtx layout.Context, f frame) {
	from := v.scroll.Y
	v.glideStep(gtx, f)
	if v.scroll.Y != from {
		v.takePointer(gtx, f)
	}
}

// easeOut is a cubic ease-out of t in [0, 1].
func easeOut(t float64) float64 { return 1 - math.Pow(1-t, 3) }
