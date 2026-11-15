package godemirror

import (
	"regexp"
	"slices"
	"time"
)

// The undo history ports @codemirror/commands 6.11.1's history.ts, without
// its effects (invertedEffects) and its JSON form, which nothing in
// godemirror has.

// IsolateHistory keeps a transaction's changes out of the undo event
// before it, after it, or both, as CodeMirror's isolateHistory annotation.
type IsolateHistory int

const (
	// IsolateBefore starts a new undo event with the transaction.
	IsolateBefore IsolateHistory = iota + 1
	// IsolateAfter starts a new undo event after the transaction.
	IsolateAfter
	// IsolateFull makes the transaction an undo event of its own.
	IsolateFull
)

// historyConfig is CodeMirror's history config. godemirror sets none: an
// Editor takes CodeMirror's defaults, and tests their own.
type historyConfig struct {
	minDepth      int
	newGroupDelay time.Duration
	joinToEvent   func(t Transaction, adjacent bool) bool
}

// defaultHistory is history()'s config with nothing set.
var defaultHistory = historyConfig{
	minDepth:      100,
	newGroupDelay: 500 * time.Millisecond,
	joinToEvent:   func(_ Transaction, adjacent bool) bool { return adjacent },
}

// maxSelectionsPerEvent is how many selections an undo event keeps for
// undoSelection, as CodeMirror's MaxSelectionsPerEvent.
const maxSelectionsPerEvent = 200

var (
	joinableUserEvent  = regexp.MustCompile(`^(input\.type|delete)($|\.)`)
	selectionUserEvent = regexp.MustCompile(`^select($|\.)`)
)

type branchName int

const (
	doneBranch branchName = iota
	undoneBranch
)

// histEvent is a group of changes undone or redone together: changes take
// the document back, mapped is the mapping of changes kept out of the
// history to apply to the events below, startSelection the selection before
// it, and selectionsAfter the selections set after it, for undoSelection.
// An event of selections alone has no changes.
type histEvent struct {
	changes         *ChangeSet
	mapped          *ChangeSet
	startSelection  *EditorSelection
	selectionsAfter []EditorSelection
}

type branch []histEvent

func (e histEvent) withSelectionsAfter(after []EditorSelection) histEvent {
	e.selectionsAfter = after
	return e
}

// eventOf is the undo event of t, nil when it changes nothing; selection is
// the selection before it, t's start selection when nil.
func eventOf(t Transaction, selection *EditorSelection) *histEvent {
	if t.Changes.Empty() {
		return nil
	}
	inverted := t.Changes.invert(t.StartState.doc)
	if selection == nil {
		selection = &t.StartState.sel
	}
	return &histEvent{changes: &inverted, startSelection: selection}
}

func updateBranch(b branch, to, maxLen int, event histEvent) branch {
	start := 0
	if to+1 > maxLen+20 {
		start = to - maxLen - 1
	}
	return append(slices.Clone(b[start:to]), event)
}

// isAdjacent reports whether b's changes touch a's, both inverted, b made
// in the document a left.
func isAdjacent(a, b ChangeSet) bool {
	var ranges []int
	a.IterChanges(func(from, to, _, _ int, _ string) { ranges = append(ranges, from, to) })
	adjacent := false
	b.IterChanges(func(_, _, from, to int, _ string) {
		for i := 0; i < len(ranges); i += 2 {
			if to >= ranges[i] && from <= ranges[i+1] {
				adjacent = true
			}
		}
	})
	return adjacent
}

func eqSelectionShape(a, b EditorSelection) bool {
	if len(a.ranges) != len(b.ranges) {
		return false
	}
	for i, r := range a.ranges {
		if r.Empty() != b.ranges[i].Empty() {
			return false
		}
	}
	return true
}

func addSelectionTo(b branch, sel EditorSelection) branch {
	if len(b) == 0 {
		return branch{{selectionsAfter: []EditorSelection{sel}}}
	}
	last := b[len(b)-1]
	sels := slices.Clone(last.selectionsAfter[max(0, len(last.selectionsAfter)-maxSelectionsPerEvent):])
	if len(sels) > 0 && sels[len(sels)-1].eq(sel, false) {
		return b
	}
	return updateBranch(b, len(b)-1, 1e9, last.withSelectionsAfter(append(sels, sel)))
}

func popSelection(b branch) branch {
	last := b[len(b)-1]
	b = slices.Clone(b)
	b[len(b)-1] = last.withSelectionsAfter(last.selectionsAfter[:len(last.selectionsAfter)-1])
	return b
}

// addMappingTo maps the top event of b through mapping, dropping it, and
// carrying the mapping on to the one below, when nothing of it is left.
func addMappingTo(b branch, mapping ChangeSet) branch {
	var selections []EditorSelection
	for n := len(b); n > 0; n-- {
		event := mapEvent(b[n-1], mapping, selections)
		if event.changes != nil && !event.changes.Empty() {
			result := slices.Clone(b[:n])
			result[n-1] = event
			return result
		}
		// An event of selections alone has no mapping; it is only ever the
		// bottom of a branch, so nothing below needs one.
		if event.mapped != nil {
			mapping = *event.mapped
		}
		selections = event.selectionsAfter
	}
	if len(selections) > 0 {
		return branch{{selectionsAfter: selections}}
	}
	return nil
}

func mapEvent(e histEvent, mapping ChangeSet, extra []EditorSelection) histEvent {
	var selections []EditorSelection
	for _, s := range e.selectionsAfter {
		selections = append(selections, s.Map(mapping))
	}
	selections = append(selections, extra...)
	if e.changes == nil {
		return histEvent{selectionsAfter: selections}
	}
	changes := e.changes.mapOnto(mapping)
	before := mapping.mapOntoBefore(*e.changes)
	full := before
	if e.mapped != nil {
		full = e.mapped.compose(before)
	}
	start := e.startSelection.Map(before)
	return histEvent{changes: &changes, mapped: &full, startSelection: &start, selectionsAfter: selections}
}

// historyState is the history of an EditorState: the events to undo and to
// redo, and the time and user event of the last transaction added.
type historyState struct {
	done, undone  branch
	prevTime      time.Time
	prevUserEvent string
}

func (h historyState) isolate() historyState {
	if h.prevTime.IsZero() {
		return h
	}
	return historyState{done: h.done, undone: h.undone}
}

func (h historyState) addChanges(event histEvent, t Transaction, conf historyConfig) historyState {
	done := h.done
	userEvent := t.UserEvent
	var last *histEvent
	if len(done) > 0 {
		last = &done[len(done)-1]
	}
	if last != nil && last.changes != nil && !last.changes.Empty() && event.changes != nil &&
		(userEvent == "" || joinableUserEvent.MatchString(userEvent)) &&
		(len(last.selectionsAfter) == 0 && t.Time.Sub(h.prevTime) < conf.newGroupDelay &&
			conf.joinToEvent(t, isAdjacent(*last.changes, *event.changes)) ||
			// A composition, past its start, always joins the event before.
			userEvent == "input.type.compose") {
		joined := event.changes.compose(*last.changes)
		done = updateBranch(done, len(done)-1, conf.minDepth,
			histEvent{changes: &joined, mapped: last.mapped, startSelection: last.startSelection})
	} else {
		done = updateBranch(done, len(done), conf.minDepth, event)
	}
	return historyState{done: done, prevTime: t.Time, prevUserEvent: userEvent}
}

func (h historyState) addSelection(sel EditorSelection, t Transaction, conf historyConfig) historyState {
	var last []EditorSelection
	if len(h.done) > 0 {
		last = h.done[len(h.done)-1].selectionsAfter
	}
	if len(last) > 0 && t.Time.Sub(h.prevTime) < conf.newGroupDelay &&
		t.UserEvent == h.prevUserEvent && selectionUserEvent.MatchString(t.UserEvent) &&
		eqSelectionShape(last[len(last)-1], sel) {
		return h
	}
	return historyState{done: addSelectionTo(h.done, sel), undone: h.undone, prevTime: t.Time, prevUserEvent: t.UserEvent}
}

func (h historyState) addMapping(mapping ChangeSet) historyState {
	return historyState{done: addMappingTo(h.done, mapping), undone: addMappingTo(h.undone, mapping),
		prevTime: h.prevTime, prevUserEvent: h.prevUserEvent}
}

// fromHistory marks a transaction an undo or a redo made: from the side it
// popped, what is left of that side, and the selection to record with the
// event it adds to the other side.
type fromHistory struct {
	side      branchName
	rest      branch
	selection EditorSelection
}

// apply is the history after t, as the history field's update.
func (h historyState) apply(t Transaction, from *fromHistory, conf historyConfig) historyState {
	if from != nil {
		other := h.done
		if from.side == doneBranch {
			other = h.undone
		}
		if event := eventOf(t, &from.selection); event != nil {
			other = updateBranch(other, len(other), conf.minDepth, *event)
		} else {
			other = addSelectionTo(other, t.StartState.sel)
		}
		if from.side == doneBranch {
			return historyState{done: from.rest, undone: other}
		}
		return historyState{done: other, undone: from.rest}
	}
	if t.IsolateHistory == IsolateFull || t.IsolateHistory == IsolateBefore {
		h = h.isolate()
	}
	if t.SkipHistory {
		if !t.Changes.Empty() {
			return h.addMapping(t.Changes.desc())
		}
		return h
	}
	if event := eventOf(t, nil); event != nil {
		h = h.addChanges(*event, t, conf)
	} else if len(t.Selection.ranges) > 0 {
		h = h.addSelection(t.StartState.sel, t, conf)
	}
	if t.IsolateHistory == IsolateFull || t.IsolateHistory == IsolateAfter {
		h = h.isolate()
	}
	return h
}

// popChanges is the transaction that undoes (side doneBranch) or redoes
// the last event, as CodeMirror's undo and redo; ok false when there is
// none, or it holds only selections.
func (h historyState) popChanges(side branchName, s EditorState) (TransactionSpec, bool) {
	b, event, selection, ok := h.top(side, s)
	if !ok || event.changes == nil {
		return TransactionSpec{}, false
	}
	rest := slices.Clone(b[:len(b)-1])
	if event.mapped != nil {
		rest = addMappingTo(rest, *event.mapped)
	}
	userEvent := "undo"
	if side == undoneBranch {
		userEvent = "redo"
	}
	changes := *event.changes
	return TransactionSpec{
		set: &changes, Selection: *event.startSelection, UserEvent: userEvent, ScrollIntoView: true,
		fromHistory: &fromHistory{side, rest, selection},
	}, true
}

// popSelectionOf is the transaction that takes the last selection of the
// last event on side back, as CodeMirror's undoSelection and
// redoSelection, or its changes when it has no selections.
func (h historyState) popSelectionOf(side branchName, s EditorState) (TransactionSpec, bool) {
	b, event, selection, ok := h.top(side, s)
	if !ok || len(event.selectionsAfter) == 0 {
		return h.popChanges(side, s)
	}
	userEvent := "select.undo"
	if side == undoneBranch {
		userEvent = "select.redo"
	}
	return TransactionSpec{
		Selection: event.selectionsAfter[len(event.selectionsAfter)-1],
		UserEvent: userEvent, ScrollIntoView: true,
		fromHistory: &fromHistory{side, popSelection(b), selection},
	}, true
}

// top is side's branch, its last event, and the selection the event
// popping it adds to the other side records, ok false when it is empty.
func (h historyState) top(side branchName, s EditorState) (b branch, event histEvent, selection EditorSelection, ok bool) {
	b = h.done
	if side == undoneBranch {
		b = h.undone
	}
	if len(b) == 0 {
		return nil, histEvent{}, EditorSelection{}, false
	}
	event = b[len(b)-1]
	switch {
	case len(event.selectionsAfter) > 0:
		selection = event.selectionsAfter[0]
	case event.startSelection != nil:
		selection = event.startSelection.mapAssoc(event.changes.invertedDesc(), 1)
	default:
		selection = s.sel
	}
	return b, event, selection, true
}

// depth is how many events side holds that change the document.
func (h historyState) depth(side branchName) int {
	b := h.done
	if side == undoneBranch {
		b = h.undone
	}
	if len(b) > 0 && b[0].changes == nil {
		return len(b) - 1
	}
	return len(b)
}
