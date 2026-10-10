package godemirror

// Align is where ScrollToLine puts its line in the view.
type Align int

const (
	// AlignNearest scrolls as little as brings the whole line into view, and
	// not at all when it is already in view.
	AlignNearest Align = iota
	// AlignCenter puts the line in the middle of the view.
	AlignCenter
	// AlignTop puts the line at the top of the view.
	AlignTop
)

type scrollTarget struct {
	line  int
	align Align
}

// ScrollToLine scrolls the view down or up so that line (0-based) sits where
// align says, as far as the document's ends allow. It takes effect on the next
// Layout, against the content shown then; a line past the end is the last
// line. It never scrolls sideways. On either side of a Split it scrolls both.
func (v *View) ScrollToLine(line int, align Align) {
	v.target = &scrollTarget{line, align}
}

// scrollToTarget applies ScrollToLine once the rows are placed in tops, and
// reports whether it scrolled.
func (v *View) scrollToTarget(f frame) bool {
	t := v.target
	if t == nil {
		return false
	}
	v.target = nil
	line := min(max(t.line, 0), len(v.doc.lines)-1)
	top := v.tops[line]
	bottom := top + (v.starts[line+1]-v.starts[line])*f.ch
	y := v.scroll.Y
	switch t.align {
	case AlignTop:
		y = top
	case AlignCenter:
		y = (top+bottom)/2 - f.size.Y/2
	case AlignNearest:
		if top < y {
			y = top
		} else if bottom > y+f.size.Y {
			y = bottom - f.size.Y
		}
	}
	maxY := max(v.tops[len(v.tops)-1]-f.size.Y, 0)
	v.scroll.Y = min(max(y, 0), maxY)
	return true
}
