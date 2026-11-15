package godemirror

import (
	"cmp"
	"fmt"
	"slices"

	"gioui.org/layout"
)

// An Editor's decorations are what a View's Content.Decorations and
// Content.Widgets are, anchored to positions instead of line numbers, so
// that they follow edits as @codemirror/view 6.43.15's line and widget
// decorations follow them when @codemirror/state's RangeSet.map maps them.

// LineDecorationAt is a LineDecoration on the line that starts at Pos, as
// CodeMirror's Decoration.line: text typed at the line's start, a line
// break included, leaves it where it is, and deleting the line break before
// the line drops it.
type LineDecorationAt struct {
	Pos        int
	Decoration LineDecoration
}

// LineWidgetAt is a LineWidget on the line Pos is on, mapped as CodeMirror
// maps Decoration.widget at Pos: with Side zero or below, text typed at Pos
// goes after it; above zero, before it. A deletion across Pos drops it.
type LineWidgetAt struct {
	Pos       int
	Side      int
	Placement Placement
	// WhileHovered shows the widget only while the pointer is over its
	// line's hover group, as LineWidget.WhileHovered.
	WhileHovered bool
	Widget       layout.Widget
}

// Decorations are an Editor's line decorations and widgets. Several line
// decorations on one line merge in order: each field one of them sets wins
// over the ones before, and their Spans join.
type Decorations struct {
	Lines   []LineDecorationAt
	Widgets []LineWidgetAt
}

// check fails for a decoration outside doc, or a line decoration that is
// not at the start of a line, as CodeMirror requires of Decoration.line.
func (d Decorations) check(s EditorState) error {
	for _, l := range d.Lines {
		if l.Pos < 0 || l.Pos > len(s.doc) || s.LineAt(l.Pos).From != l.Pos {
			return fmt.Errorf("godemirror: a line decoration at %d is not at the start of a line", l.Pos)
		}
	}
	for _, w := range d.Widgets {
		if w.Pos < 0 || w.Pos > len(s.doc) {
			return fmt.Errorf("godemirror: a widget at %d is outside a document of length %d", w.Pos, len(s.doc))
		}
		if err := s.checkBoundary(w.Pos); err != nil {
			return err
		}
	}
	return nil
}

// sorted is d in document order, as a RangeSet keeps it; decorations at one
// place keep the order they were given in.
func (d Decorations) sorted() Decorations {
	lines, widgets := slices.Clone(d.Lines), slices.Clone(d.Widgets)
	slices.SortStableFunc(lines, func(a, b LineDecorationAt) int { return cmp.Compare(a.Pos, b.Pos) })
	slices.SortStableFunc(widgets, func(a, b LineWidgetAt) int { return cmp.Compare(a.Pos, b.Pos) })
	return Decorations{lines, widgets}
}

// mapThrough is d moved through c, as RangeSet.map moves line and widget
// decorations: a line decoration with the MapMode TrackBefore of
// LineDecoration, a widget with TrackDel, each on its side.
func (d Decorations) mapThrough(c ChangeSet) Decorations {
	if c.Empty() {
		return d
	}
	var out Decorations
	for _, l := range d.Lines {
		if pos, ok := c.MapPosTrack(l.Pos, -1, MapTrackBefore); ok {
			l.Pos = pos
			out.Lines = append(out.Lines, l)
		}
	}
	for _, w := range d.Widgets {
		assoc := -1
		if w.Side > 0 {
			assoc = 1
		}
		if pos, ok := c.MapPosTrack(w.Pos, assoc, MapTrackDel); ok {
			w.Pos = pos
			out.Widgets = append(out.Widgets, w)
		}
	}
	return out.sorted()
}

// Decorations are the editor state's decorations, mapped through every
// change since they were set. It is a copy.
func (s EditorState) Decorations() Decorations {
	return Decorations{slices.Clone(s.decos.Lines), slices.Clone(s.decos.Widgets)}
}

// lineDecorations are s's decorations as a Content holds them, by line.
func (s EditorState) lineDecorations() (decorations []LineDecoration, widgets []LineWidget) {
	for _, l := range s.decos.Lines {
		line := s.LineAt(l.Pos).Number - 1
		for len(decorations) <= line {
			decorations = append(decorations, LineDecoration{})
		}
		decorations[line] = decorations[line].merge(l.Decoration)
	}
	for _, w := range s.decos.Widgets {
		widgets = append(widgets, LineWidget{
			Line: s.LineAt(w.Pos).Number - 1, Placement: w.Placement, WhileHovered: w.WhileHovered, Widget: w.Widget,
		})
	}
	return decorations, widgets
}

// merge is d with the fields o sets over it, and o's spans after d's.
func (d LineDecoration) merge(o LineDecoration) LineDecoration {
	zero := LineDecoration{}.Background
	if o.Background != zero {
		d.Background = o.Background
	}
	if o.GutterBackground != zero {
		d.GutterBackground = o.GutterBackground
	}
	if o.GutterStrip != zero {
		d.GutterStrip = o.GutterStrip
	}
	if o.Rule != zero {
		d.Rule = o.Rule
	}
	if o.Hatch != zero {
		d.Hatch = o.Hatch
	}
	if o.HoverGroup != 0 {
		d.HoverGroup = o.HoverGroup
	}
	if o.HoverBackground != zero {
		d.HoverBackground = o.HoverBackground
	}
	d.Spans = append(slices.Clone(d.Spans), o.Spans...)
	return d
}
