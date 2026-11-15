package godemirror

import (
	"image"

	"gioui.org/io/key"
)

// mouseSelection ports @codemirror/view 6.43.15's basicMouseSelection
// (input.ts): what a press, and the drag after it, select in an Editor.
// Dragging a selection elsewhere, CodeMirror's drag and drop, is not ported:
// a press inside the selection selects at once.
type mouseSelection struct {
	e *Editor
	// start is where the press was, with its side, startSel the selection
	// before it, and clicks its count.
	start       int
	startAssoc  int
	startSel    EditorSelection
	clicks      int
	extend      bool
	addsToRange bool
}

// posAndSide is the position under at, a point in the view, and the side of
// it the point is on.
func (e *Editor) posAndSide(f frame, at image.Point) (pos, assoc int) {
	v := e.view
	x := at.X - f.g.textX + v.scrollX()
	return e.posAtCoords(x, at.Y+v.scroll.Y)
}

func (m *mouseSelection) press(f frame, at image.Point, clicks int, mods key.Modifiers) {
	e := m.e
	m.start, m.startAssoc = e.posAndSide(f, at)
	m.startSel = e.state.sel
	m.clicks = clicks
	m.extend = mods.Contain(key.ModShift)
	m.addsToRange = e.state.conf.AllowMultipleSelections && mods.Contain(key.ModShortcut)
	m.selectAt(m.start, m.startAssoc)
}

func (m *mouseSelection) drag(f frame, at image.Point) { m.selectAt(m.e.posAndSide(f, at)) }

// mapThrough keeps the press where it was through changes made while it is
// held.
func (m *mouseSelection) mapThrough(c ChangeSet) {
	m.start = c.MapPos(m.start, -1)
	m.startSel = m.startSel.Map(c)
}

func (m *mouseSelection) selectAt(cur, assoc int) {
	m.e.mustDispatch(TransactionSpec{Selection: m.get(cur, assoc), UserEvent: "select.pointer"})
}

// get is basicMouseSelection's get: the selection the pointer at cur makes.
func (m *mouseSelection) get(cur, assoc int) EditorSelection {
	s := m.e.state
	r := s.rangeForClick(cur, assoc, m.clicks)
	if m.start != cur && !m.extend {
		startRange := s.rangeForClick(m.start, m.startAssoc, m.clicks)
		from, to := min(startRange.from, r.from), max(startRange.to, r.to)
		// CodeMirror passes the range's assoc where the goal column goes, a
		// pixel or less from the start of the line: column 0.
		if from < r.from {
			r = rangeWith(from, to, 0, 0)
		} else {
			r = rangeWith(to, from, 0, 0)
		}
	}
	switch {
	case m.extend:
		return m.startSel.ReplaceRange(m.startSel.Main().extend(r.from, r.to, r.Assoc()), m.startSel.main)
	case m.addsToRange && m.clicks == 1 && len(m.startSel.ranges) > 1:
		if removed, ok := removeRangeAround(m.startSel, cur); ok {
			return removed
		}
		return m.startSel.AddRange(r)
	case m.addsToRange:
		return m.startSel.AddRange(r)
	}
	return NewSelection([]SelectionRange{r}, 0)
}

// rangeForClick is the range a press at pos selects: a cursor for one click,
// the group under it for two, its line and line break for three or more.
func (s EditorState) rangeForClick(pos, bias, clicks int) SelectionRange {
	switch {
	case clicks == 1:
		return cursorAssoc(pos, bias)
	case clicks == 2:
		return s.groupAt(pos, bias)
	}
	l := s.LineAt(pos)
	to := l.To
	if to < len(s.doc) {
		to++
	}
	return Range(l.From, to)
}

// removeRangeAround is sel without the range that holds pos, ok false when
// none does.
func removeRangeAround(sel EditorSelection, pos int) (EditorSelection, bool) {
	for i, r := range sel.ranges {
		if r.from <= pos && r.to >= pos {
			ranges := append(sel.Ranges()[:i:i], sel.ranges[i+1:]...)
			main := sel.main
			switch {
			case main == i:
				main = 0
			case main > i:
				main--
			}
			return NewSelection(ranges, main), true
		}
	}
	return EditorSelection{}, false
}
