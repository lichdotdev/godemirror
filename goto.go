package godemirror

import (
	"context"
	"slices"
	"unicode/utf8"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
)

// LineColumn is a place in a document: Line counted from 0, and Column in
// UTF-8 bytes from the start of the line.
type LineColumn struct {
	Line, Column int
}

// Location is a span of a document: URI names the document, empty for the
// editor's own, and Start and End are its ends.
type Location struct {
	URI        string
	Start, End LineColumn
}

// Shortcut is a key pressed with exactly Modifiers held; the zero Shortcut
// is no key.
type Shortcut struct {
	Name      key.Name
	Modifiers key.Modifiers
}

// The keys a new Editor goes to a type definition, an implementation and
// the references with, as VS Code binds the last two.
var (
	DefaultTypeDefinitionKey = Shortcut{key.NameF12, key.ModShortcut | key.ModShift}
	DefaultImplementationKey = Shortcut{key.NameF12, key.ModShortcut}
	DefaultReferencesKey     = Shortcut{key.NameF12, key.ModShift}
)

// GoTo finds where to go from the name at pos, a position in s: a
// definition, a type definition, an implementation. ok false finds
// nothing.
type GoTo func(ctx context.Context, s EditorState, pos int) (loc Location, ok bool)

// definitionAnswer is what a GoTo call found.
type definitionAnswer struct {
	loc Location
	ok  bool
}

// goTo asks hook where to go from the name at pos; the answer moves the
// cursor, or goes to OnDefinition, in a later frame.
func (e *Editor) goTo(hook GoTo, pos int) {
	if hook == nil {
		return
	}
	e.definition.stop()
	e.definition = nil
	s := e.state
	e.later(func() {
		e.definition.stop()
		e.definition = startCall(func(ctx context.Context) definitionAnswer {
			loc, ok := hook(ctx, s, pos)
			return definitionAnswer{loc, ok}
		})
	})
}

// takeDefinition acts on a GoTo's answer once it comes: a place in the
// document moves the cursor there, another document goes to OnDefinition
// after the frame.
func (e *Editor) takeDefinition(gtx layout.Context) {
	if e.definition == nil {
		return
	}
	a, ok := e.definition.poll(gtx)
	if !ok {
		return
	}
	e.definition = nil
	switch {
	case !a.ok:
	case a.loc.URI == "":
		pos := e.posOfLineColumn(a.loc.Start)
		e.mustDispatch(TransactionSpec{Selection: SingleSelection(pos, pos), ScrollIntoView: true, UserEvent: "select"})
	default:
		e.elsewhere = append(e.elsewhere, a.loc)
	}
}

// posOfLineColumn is the position of p in the editor's document, clamped
// to it.
func (e *Editor) posOfLineColumn(p LineColumn) int {
	s := e.state
	line := s.Line(min(max(p.Line+1, 1), s.Lines()))
	return e.clampPos(line.From + min(max(p.Column, 0), len(line.Text)))
}

// reportDefinitions hands the places found in other documents to
// OnDefinition, and the references found to OnReferences, after the frame.
func (e *Editor) reportDefinitions() {
	found, refs := e.elsewhere, e.foundRefs
	e.elsewhere, e.foundRefs = nil, nil
	if e.OnDefinition != nil {
		for _, loc := range found {
			e.OnDefinition(loc)
		}
	}
	if refs != nil && e.OnReferences != nil {
		e.OnReferences(refs)
	}
}

// goToBinding is a configurable go-to key and what it runs at the cursor.
type goToBinding struct {
	key Shortcut
	run func(pos int)
}

// goToKeys are the configurable go-to keys.
func (e *Editor) goToKeys() []goToBinding {
	return []goToBinding{
		{e.TypeDefinitionKey, func(pos int) { e.goTo(e.TypeDefinition, pos) }},
		{e.ImplementationKey, func(pos int) { e.goTo(e.Implementation, pos) }},
		{e.ReferencesKey, e.findReferences},
	}
}

// goToFilters are the key filters of the go-to keys that are set.
func (e *Editor) goToFilters() []event.Filter {
	var out []event.Filter
	for _, k := range e.goToKeys() {
		if k.key.Name != "" {
			out = append(out, key.Filter{Focus: e.view, Name: k.key.Name, Required: k.key.Modifiers})
		}
	}
	return out
}

// goToKey runs the go-to key ev is, and reports whether it was one.
func (e *Editor) goToKey(ev key.Event) bool {
	for _, k := range e.goToKeys() {
		if k.key.Name != "" && k.key == (Shortcut{ev.Name, ev.Modifiers}) {
			k.run(e.state.sel.Main().Head())
			return true
		}
	}
	return false
}

// findReferences asks References for the references to the name at pos.
func (e *Editor) findReferences(pos int) {
	hook := e.References
	if hook == nil {
		return
	}
	e.references.stop()
	e.references = nil
	s := e.state
	e.later(func() {
		e.references.stop()
		e.references = startCall(func(ctx context.Context) []Location { return hook(ctx, s, pos) })
	})
}

// takeReferences highlights the references References found in the
// document, and keeps them all for OnReferences after the frame.
func (e *Editor) takeReferences(gtx layout.Context) {
	if e.references == nil {
		return
	}
	refs, ok := e.references.poll(gtx)
	if !ok {
		return
	}
	e.references = nil
	e.occurrences = nil
	for _, l := range refs {
		if l.URI == "" {
			e.occurrences = append(e.occurrences, Range(e.posOfLineColumn(l.Start), e.posOfLineColumn(l.End)))
		}
	}
	slices.SortFunc(e.occurrences, func(a, b SelectionRange) int { return a.from - b.from })
	e.foundRefs = append([]Location{}, refs...)
}

// occurrenceSpans are the highlighted references on line (from 0), in its
// runes, as the view's spans.
func (e *Editor) occurrenceSpans(line int) []Span {
	if len(e.occurrences) == 0 || line >= e.state.Lines() {
		return nil
	}
	l := e.state.Line(line + 1)
	var spans []Span
	for _, o := range e.occurrences {
		if o.to < l.From || o.from > l.To {
			continue
		}
		from, to := max(o.from, l.From)-l.From, min(o.to, l.To)-l.From
		spans = append(spans, Span{
			From:  utf8.RuneCountInString(l.Text[:from]),
			To:    utf8.RuneCountInString(l.Text[:to]),
			Color: e.Theme.Occurrence,
		})
	}
	return spans
}
