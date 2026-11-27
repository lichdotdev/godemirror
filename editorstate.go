package godemirror

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// EditorState, Transaction and the functions that make one from the other
// port what @codemirror/state 6.7.6's state.ts and transaction.ts do for a
// document and its selection. Facets, fields, effects, annotations and
// transaction filters are not ported: the options CodeMirror sets through
// facets are fields of EditorStateConfig.

// EditorStateConfig is what an Editor starts from, as CodeMirror's
// EditorState.create takes it.
type EditorStateConfig struct {
	// Doc is the document. Its line breaks (LF, CRLF or CR, or only
	// LineSeparator when set) become "\n" in EditorState.Doc, and a final
	// line break starts an empty last line, as in CodeMirror.
	Doc string
	// Selection is where the selection starts; the zero EditorSelection puts
	// a cursor at the start.
	Selection EditorSelection
	// AllowMultipleSelections keeps every range of a selection, as
	// CodeMirror's facet of the same name does; without it a selection is
	// cut to its main range.
	AllowMultipleSelections bool
	// LineSeparator, when set, is the only line break Doc and inserted text
	// are split on, and what SliceDoc joins lines with: "\r\n" keeps a CRLF
	// file CRLF.
	LineSeparator string
	// IndentUnit is one level of indentation, as CodeMirror's indentUnit
	// facet: a run of spaces, or of tabs, and two spaces when empty. Enter
	// writes a new line's indentation in it.
	IndentUnit string
	// Filename picks the language Doc is highlighted as, as Content.Filename.
	Filename string
	// StyleTags add rules to the Lezer grammars they name, as
	// Content.StyleTags.
	StyleTags []StyleTags
	// Decorations are the line decorations and widgets the editor starts
	// with.
	Decorations Decorations

	// history is the undo history's config, CodeMirror's defaults when
	// zero.
	history historyConfig
}

// EditorState is a document and its selection at one moment. It never
// changes: a transaction makes a new one. Positions are byte offsets into
// Doc, whose lines are joined by "\n". Where the read-only views count a
// column in runes (Pos), an EditorState counts bytes.
type EditorState struct {
	doc string
	// lines are the offsets each line starts at.
	lines []int
	sel   EditorSelection
	conf  *EditorStateConfig
	hist  historyState
	// decos are the decorations, and decosGen counts the times they
	// changed, for an Editor to know when to show them again.
	decos    Decorations
	decosGen int
}

func newEditorState(c EditorStateConfig) (EditorState, error) {
	conf := c
	conf.Doc, conf.Selection, conf.Decorations = "", EditorSelection{}, Decorations{}
	unit, err := indentUnitOf(c.IndentUnit)
	if err != nil {
		return EditorState{}, err
	}
	conf.IndentUnit = unit
	if conf.history.joinToEvent == nil {
		conf.history = defaultHistory
	}
	s := EditorState{doc: normalizeBreaks(c.Doc, c.LineSeparator), conf: &conf}
	s.lines = lineStarts(s.doc)
	s.sel = c.Selection
	if len(s.sel.ranges) == 0 {
		s.sel = SingleSelection(0, 0)
	}
	if err := s.checkSelection(s.sel); err != nil {
		return EditorState{}, err
	}
	if err := c.Decorations.check(s); err != nil {
		return EditorState{}, err
	}
	s.decos = c.Decorations.sorted()
	if !conf.AllowMultipleSelections {
		s.sel = s.sel.AsSingle()
	}
	return s, nil
}

func lineStarts(doc string) []int {
	starts := []int{0}
	for at := 0; ; {
		i := strings.IndexByte(doc[at:], '\n')
		if i < 0 {
			return starts
		}
		at += i + 1
		starts = append(starts, at)
	}
}

// Doc is the document, its lines joined by "\n" whatever LineSeparator is:
// a position is a byte offset into it.
func (s EditorState) Doc() string { return s.doc }

// SliceDoc is the document's bytes [from, to), its lines joined by
// LineSeparator, or by "\n" without one.
func (s EditorState) SliceDoc(from, to int) string {
	text := s.doc[from:to]
	if sep := s.conf.LineSeparator; sep != "" && sep != "\n" {
		return strings.ReplaceAll(text, "\n", sep)
	}
	return text
}

// Selection is the selection.
func (s EditorState) Selection() EditorSelection { return s.sel }

// IndentUnit is one level of indentation in s, as CodeMirror's
// getIndentUnit reads the indentUnit facet: spaces or tabs.
func (s EditorState) IndentUnit() string { return s.conf.IndentUnit }

// Lines is the number of lines, at least one.
func (s EditorState) Lines() int { return len(s.lines) }

// Line is a line of the document: its number, from 1, and the bytes [From,
// To) of Doc that hold its Text, its line break left out.
type Line struct {
	Number   int
	From, To int
	Text     string
}

// Line is line n, counted from 1 as CodeMirror's Text.line counts. n must be
// in [1, Lines()].
func (s EditorState) Line(n int) Line {
	from, to := s.lines[n-1], len(s.doc)
	if n < len(s.lines) {
		to = s.lines[n] - 1
	}
	return Line{Number: n, From: from, To: to, Text: s.doc[from:to]}
}

// LineAt is the line pos is on. pos must be in [0, len(Doc())].
func (s EditorState) LineAt(pos int) Line {
	i, found := slices.BinarySearch(s.lines, pos)
	if !found {
		i--
	}
	return s.Line(i + 1)
}

// RangeChange is what the function ChangeByRange runs for each range
// returns: the changes it makes, as positions in the document before any of
// them, and where the range goes, in the document those changes alone make.
type RangeChange struct {
	Changes []ChangeSpec
	Range   SelectionRange
}

// ChangeByRange runs f for each range of the selection and merges what it
// returns into one transaction, each range's changes mapped through the
// others, as CodeMirror's EditorState.changeByRange does.
func (s EditorState) ChangeByRange(f func(SelectionRange) RangeChange) TransactionSpec {
	first := f(s.sel.ranges[0])
	changes, err := s.changes(first.Changes)
	if err != nil {
		return TransactionSpec{err: err}
	}
	ranges := []SelectionRange{first.Range}
	for _, r := range s.sel.ranges[1:] {
		result := f(r)
		own, err := s.changes(result.Changes)
		if err != nil {
			return TransactionSpec{err: err}
		}
		mapped := own.mapOnto(changes)
		for j := range ranges {
			ranges[j] = ranges[j].Map(mapped)
		}
		ranges = append(ranges, result.Range.Map(changes.mapOntoBefore(own)))
		changes = changes.compose(mapped)
	}
	return TransactionSpec{set: &changes, Selection: NewSelection(ranges, s.sel.main)}
}

// ReplaceSelection replaces every range of the selection with text, leaving
// a cursor after each, as CodeMirror's EditorState.replaceSelection does.
func (s EditorState) ReplaceSelection(text string) TransactionSpec {
	n := len(normalizeBreaks(text, s.conf.LineSeparator))
	return s.ChangeByRange(func(r SelectionRange) RangeChange {
		return RangeChange{
			Changes: []ChangeSpec{{From: r.from, To: r.to, Insert: text}},
			Range:   cursorAssoc(r.from+n, -1),
		}
	})
}

func (s EditorState) changes(specs []ChangeSpec) (ChangeSet, error) {
	return changeSetOf(specs, len(s.doc), s.conf.LineSeparator, nil)
}

// checkSelection fails for a range outside the document or inside a
// character.
func (s EditorState) checkSelection(sel EditorSelection) error {
	for _, r := range sel.ranges {
		if r.from < 0 || r.to > len(s.doc) {
			return fmt.Errorf("godemirror: selection range %d to %d points outside a document of length %d", r.from, r.to, len(s.doc))
		}
		if err := s.checkBoundary(r.from); err != nil {
			return err
		}
		if err := s.checkBoundary(r.to); err != nil {
			return err
		}
	}
	return nil
}

func (s EditorState) checkBoundary(pos int) error {
	if pos < len(s.doc) && !utf8.RuneStart(s.doc[pos]) {
		return fmt.Errorf("godemirror: position %d is inside a character", pos)
	}
	return nil
}

// TransactionSpec describes a transaction for Editor.Dispatch, as
// CodeMirror's TransactionSpec.
type TransactionSpec struct {
	// Changes are made to the document at once, positions all in the
	// document before them. Changes given out of order are applied in the
	// order given, each mapped over those before it.
	Changes []ChangeSpec
	// Selection is the selection after the transaction, in the document
	// after the changes. The zero EditorSelection maps the one before through
	// the changes.
	Selection EditorSelection
	// UserEvent names what made the transaction, in CodeMirror's dotted
	// names ("input.type", "delete.backward", "select.pointer"), for
	// Transaction.IsUserEvent.
	UserEvent string
	// ScrollIntoView scrolls the main range's head into view on the next
	// Layout.
	ScrollIntoView bool
	// SkipHistory keeps the transaction out of the undo history, as
	// CodeMirror's Transaction.addToHistory set to false: undo does not
	// take its changes back, and maps the history through them.
	SkipHistory bool
	// IsolateHistory keeps the transaction's changes out of the undo events
	// next to it; by default, changes made close together in time and place
	// are undone as one, as CodeMirror's history groups them.
	IsolateHistory IsolateHistory
	// Time is when the transaction happened, which groups undo events; zero
	// is now.
	Time time.Time
	// Decorations, when set, replace the editor's decorations after the
	// changes, at positions in the document they make; when nil, the
	// decorations are mapped through the changes. A caller recomputes them
	// in OnUpdate, as a CodeMirror state field does with its effects.
	Decorations *Decorations

	// set is changes ChangeByRange already merged, made before Changes.
	set *ChangeSet
	// scrollTo is a position to scroll into view, as CodeMirror's
	// EditorView.scrollIntoView effect, instead of the main range's head.
	scrollTo    *int
	fromHistory *fromHistory
	err         error
}

// Transaction is one update of an EditorState, as CodeMirror's Transaction.
type Transaction struct {
	StartState EditorState
	// State is the state the transaction makes.
	State EditorState
	// Changes take the document of StartState to the one of State.
	Changes ChangeSet
	// Selection is the selection the transaction set, or the zero
	// EditorSelection when it only mapped the one before.
	Selection      EditorSelection
	UserEvent      string
	ScrollIntoView bool
	// SkipHistory, IsolateHistory and Time are the spec's, Time now when it
	// gave none.
	SkipHistory    bool
	IsolateHistory IsolateHistory
	Time           time.Time

	scrollTo *int
}

// DocChanged reports whether the transaction changes the document.
func (t Transaction) DocChanged() bool { return !t.Changes.Empty() }

// IsUserEvent reports whether the transaction's UserEvent is event or one of
// its subevents: "select" covers "select.pointer".
func (t Transaction) IsUserEvent(event string) bool {
	e := t.UserEvent
	return e == event || len(e) > len(event) && strings.HasPrefix(e, event) && e[len(event)] == '.'
}

// update is the transaction spec makes from s.
func (s EditorState) update(spec TransactionSpec) (Transaction, error) {
	if spec.err != nil {
		return Transaction{}, spec.err
	}
	changes, err := changeSetOf(spec.Changes, len(s.doc), s.conf.LineSeparator, spec.set)
	if err != nil {
		return Transaction{}, err
	}
	var bad error
	changes.IterChanges(func(fromA, toA, _, _ int, _ string) {
		bad = cmp.Or(bad, s.checkBoundary(fromA), s.checkBoundary(toA))
	})
	if bad != nil {
		return Transaction{}, bad
	}
	next := EditorState{doc: changes.apply(s.doc), conf: s.conf}
	next.lines = lineStarts(next.doc)
	next.sel = spec.Selection
	if len(next.sel.ranges) == 0 {
		next.sel = s.sel.Map(changes)
	} else if err := next.checkSelection(next.sel); err != nil {
		return Transaction{}, err
	}
	if !s.conf.AllowMultipleSelections {
		next.sel = next.sel.AsSingle()
	}
	next.decos, next.decosGen = s.decos.mapThrough(changes), s.decosGen
	if !changes.Empty() {
		next.decosGen++
	}
	if spec.Decorations != nil {
		if err := spec.Decorations.check(next); err != nil {
			return Transaction{}, err
		}
		next.decos, next.decosGen = spec.Decorations.sorted(), s.decosGen+1
	}
	t := Transaction{
		StartState: s, State: next, Changes: changes, Selection: spec.Selection,
		UserEvent: spec.UserEvent, ScrollIntoView: spec.ScrollIntoView, SkipHistory: spec.SkipHistory,
		IsolateHistory: spec.IsolateHistory, Time: spec.Time, scrollTo: spec.scrollTo,
	}
	if t.Time.IsZero() {
		t.Time = time.Now()
	}
	t.State.hist = s.hist.apply(t, spec.fromHistory, s.conf.history)
	return t, nil
}
