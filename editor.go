package godemirror

import (
	"time"
	"unicode/utf8"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/glyphs"
)

// Editor is a View whose document can be changed, CodeMirror's EditorView
// over an EditorState: the user types, selects, and copies, cuts and pastes
// with the keys of CodeMirror's standard keymap; a caller changes it with
// Dispatch, and hears of every change through OnUpdate. Its zero value is not
// usable; build one with NewEditor or NewEditorFromFonts.
//
// Like a View, an Editor belongs to the goroutine that lays it out.
type Editor struct {
	// Wrap breaks lines at the editor's width instead of scrolling sideways.
	Wrap bool
	// Theme colors the editor; it starts as DarkTheme.
	Theme Theme
	// RectangularSelection makes a drag with Alt held select a rectangle, a
	// range per line, as CodeMirror's rectangularSelection does; it needs
	// AllowMultipleSelections to keep more than one.
	RectangularSelection bool
	// CrosshairCursor turns the pointer into a crosshair while Alt is held,
	// as CodeMirror's crosshairCursor does, a hint that a drag selects a
	// rectangle.
	CrosshairCursor bool
	// OnUpdate, when set, is called at the end of Layout with the
	// transactions dispatched since the last call, if any, as CodeMirror's
	// updateListener: never while the editor paints, so it may dispatch
	// again, and that transaction is reported after the next frame.
	OnUpdate func(Update)

	view  *View
	state EditorState
	// shown is the state on screen, and unshown the changes from it to
	// state; mark is the selection the view was given, main and others.
	shown   EditorState
	unshown ChangeSet
	mark    []selection
	// reported is the state the next Update starts from, and since the
	// transactions after it.
	reported EditorState
	since    []Transaction

	// f is the frame Layout last laid out, which the motions read.
	f frame
	// scrollPending is a ScrollIntoView still to do; keepOnScreen, set by a
	// page motion, is where on screen the main cursor's row stays instead.
	scrollPending bool
	keepOnScreen  *int
	// scrollTarget, when set, is where scrollPending scrolls to instead of
	// the main range's head.
	scrollTarget *int
	mouse        mouseSelection
	ime          imeState
	// blinkStart is when the caret's blink last started over, and
	// selectionSet whether a transaction set the selection since.
	blinkStart   time.Time
	selectionSet bool
	// now, while Layout runs, is its frame's time, which the transactions it
	// dispatches happened at.
	now time.Time
	// altDown is whether Alt is held, as the last key or pointer event said,
	// and pointerMods the modifiers of the last pointer event seen.
	altDown     bool
	pointerMods key.Modifiers
}

// Update is what an Editor reports after a frame, as CodeMirror's
// ViewUpdate: the transactions dispatched since the last one, and the
// changes they made together.
type Update struct {
	StartState, State EditorState
	// Changes take the document of StartState to the one of State.
	Changes      ChangeSet
	Transactions []Transaction
}

// DocChanged reports whether the document changed.
func (u Update) DocChanged() bool { return !u.Changes.Empty() }

// SelectionSet reports whether a transaction set the selection.
func (u Update) SelectionSet() bool {
	for _, t := range u.Transactions {
		if len(t.Selection.ranges) > 0 {
			return true
		}
	}
	return false
}

// NewEditor builds an empty editor drawing family at size, resolved as New
// resolves it.
func NewEditor(family string, size unit.Sp) (*Editor, error) {
	r, err := glyphs.New(family)
	if err != nil {
		return nil, err
	}
	return newEditor(newView(r, size)), nil
}

// NewEditorFromFonts builds an empty editor drawing fonts at size, as
// NewFromFonts does.
func NewEditorFromFonts(fonts Fonts, size unit.Sp) (*Editor, error) {
	r, err := rasterizerOf(fonts)
	if err != nil {
		return nil, err
	}
	return newEditor(newView(r, size)), nil
}

func newEditor(v *View) *Editor {
	e := &Editor{Theme: v.Theme, view: v}
	e.mouse.e = e
	v.selecting = &e.mouse
	if err := e.SetState(EditorStateConfig{}); err != nil {
		panic(err)
	}
	return e
}

// SetState replaces the editor's state with a new one made from c, shown from
// its top, as CodeMirror's EditorView.setState: OnUpdate does not hear of it,
// and transactions not yet reported are dropped. It fails, leaving the editor
// as it was, when c.Selection is outside c.Doc or inside one of its
// characters.
func (e *Editor) SetState(c EditorStateConfig) error {
	s, err := newEditorState(c)
	if err != nil {
		return err
	}
	e.state, e.shown, e.reported = s, s, s
	e.since, e.mark = nil, nil
	e.unshown = emptyChangeSet(len(s.doc))
	e.view.SetContent(s.content())
	return nil
}

// content is what the view shows of s. A final line end starts no line of a
// Content's Text, so one more is added: an empty last line stays a line.
func (s EditorState) content() Content {
	return Content{Filename: s.conf.Filename, Text: s.doc + "\n", StyleTags: s.conf.StyleTags}
}

// State is the editor's current state, with every transaction dispatched so
// far, shown or not.
func (e *Editor) State() EditorState { return e.state }

// Dispatch applies spec to the editor's state as one transaction. The new
// state is State at once and shows on the next Layout, which reports it to
// OnUpdate; outside of OnUpdate, ask the window for that frame. It fails,
// changing nothing, when a change or a selection range of spec is outside
// the document, inside a character, or a change's From is after its To.
func (e *Editor) Dispatch(spec TransactionSpec) error {
	if spec.Time.IsZero() {
		spec.Time = e.now
	}
	t, err := e.state.update(spec)
	if err != nil {
		return err
	}
	if e.view.press.dragging && t.DocChanged() {
		e.mouse.mapThrough(t)
	}
	e.state = t.State
	e.unshown = e.unshown.compose(t.Changes)
	e.since = append(e.since, t)
	e.scrollPending = e.scrollPending || t.ScrollIntoView
	if t.scrollTo != nil {
		e.scrollPending, e.scrollTarget = true, t.scrollTo
	}
	e.selectionSet = e.selectionSet || len(t.Selection.ranges) > 0
	return nil
}

// Undo takes the last undo event back, as CodeMirror's undo: changes made
// close together are one event, unless a transaction isolated them. It
// reports whether there was one to undo.
func (e *Editor) Undo() bool { return e.dispatchHistory(e.state.hist.popChanges(doneBranch, e.state)) }

// Redo makes the last undone event again, as CodeMirror's redo, and reports
// whether there was one.
func (e *Editor) Redo() bool {
	return e.dispatchHistory(e.state.hist.popChanges(undoneBranch, e.state))
}

// UndoDepth is how many events Undo can take back, as CodeMirror's
// undoDepth.
func (e *Editor) UndoDepth() int { return e.state.hist.depth(doneBranch) }

// RedoDepth is how many events Redo can make again, as CodeMirror's
// redoDepth.
func (e *Editor) RedoDepth() int { return e.state.hist.depth(undoneBranch) }

func (e *Editor) dispatchHistory(spec TransactionSpec, ok bool) bool {
	if ok {
		e.mustDispatch(spec)
	}
	return ok
}

func (e *Editor) report(gtx layout.Context) {
	if len(e.since) == 0 {
		return
	}
	u := Update{StartState: e.reported, State: e.state, Transactions: e.since}
	u.Changes = emptyChangeSet(len(e.reported.doc))
	for _, t := range e.since {
		u.Changes = u.Changes.compose(t.Changes)
	}
	e.reported, e.since = e.state, nil
	if e.OnUpdate == nil {
		return
	}
	e.OnUpdate(u)
	if len(e.since) > 0 {
		gtx.Execute(op.InvalidateCmd{})
	}
}

// cellAt is the place between the view's cells that offset, a position in
// the editor's state, names.
func (e *Editor) cellAt(offset int) pos {
	l := e.state.LineAt(offset)
	return e.view.cellPos(Pos{Line: l.Number - 1, Col: utf8.RuneCountInString(l.Text[:offset-l.From])})
}

// offsetAt is the position in the editor's state of p, a place between the
// view's cells; inside a tab it is after it.
func (e *Editor) offsetAt(p pos) int {
	rp := e.view.runePos(p)
	l := e.state.Line(rp.Line + 1)
	off := 0
	for range rp.Col {
		_, n := utf8.DecodeRuneInString(l.Text[off:])
		off += n
	}
	return l.From + off
}
