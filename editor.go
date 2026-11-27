package godemirror

import (
	"context"
	"sync"
	"time"
	"unicode/utf8"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
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
	// OnScroll is View.OnScroll of the editor: a cursor kept in view is the
	// program's scroll, and does not call it.
	OnScroll func()
	// ReduceMotion is View.ReduceMotion of the editor: page motions and far
	// cursor moves scroll at once instead of gliding.
	ReduceMotion bool
	// FormatNumber, when set, spells the gutter's number of line lineNo
	// (counted from 1) in state, the one on screen, as the formatNumber
	// option of @codemirror/view 6.43.15's lineNumbers: relative numbers
	// read the cursor's line from it. The gutter stays as wide as the line
	// count's own number, so a longer text is cut.
	FormatNumber func(lineNo int, state EditorState) string
	// OnUpdate, when set, is called at the end of Layout with the
	// transactions dispatched since the last call, if any, as CodeMirror's
	// updateListener: never while the editor paints, so it may dispatch
	// again, and that transaction is reported after the next frame.
	OnUpdate func(Update)
	// OnKey, when set, sees every key the editor takes, pressed or
	// released, and every text Gio's text input types into it, before the
	// editor acts on it, as a keydown handler of @codemirror/view 6.43.15's
	// EditorView.domEventHandlers: returning true keeps the editor from
	// acting (no keymap command, no clipboard key, no typed text), as the
	// handler's true prevents the default. ev is a key.Event or a
	// key.EditEvent, and Gio sends a key's text apart from the key: taking
	// the key does not take the text, so a caller that types for itself
	// takes both. While OnKey is set the focused editor takes every key, Tab
	// and Shift+Tab included, which then stop moving focus; a key the app
	// binds for itself reaches the app only when the app reads it before
	// the editor's Layout. OnKey may Dispatch.
	OnKey func(gtx layout.Context, ev event.Event) bool
	// Hover, when set, tells what to show over the name at pos, a position
	// in s, once the pointer has rested on it for half a second, as
	// Markdown: the popup shows it above the name, with the diagnostics
	// there. It runs on a goroutine of its own, and ctx is cancelled when the
	// pointer leaves first; an empty answer shows nothing.
	Hover func(ctx context.Context, s EditorState, pos int) (markdown string)
	// Definition, when set, finds where the name at pos, a position in s,
	// is defined, for Ctrl+click and F12; ok false finds nothing. It runs on
	// a goroutine of its own. A Location in the editor's own document
	// moves the cursor there; one in another goes to OnDefinition. While it
	// is set, the name under the pointer shows as a link with Ctrl held,
	// and Ctrl+click goes to its definition instead of adding a cursor.
	Definition GoTo
	// TypeDefinition and Implementation, when set, find where the type of
	// the name at pos is defined, and an implementation of it, for their
	// keys (TypeDefinitionKey, ImplementationKey); they go where they find
	// as Definition goes.
	TypeDefinition, Implementation GoTo
	// OnDefinition, when set, is called at the end of Layout with a place
	// Definition, TypeDefinition or Implementation found in another
	// document, for the app to open.
	OnDefinition func(Location)
	// References, when set, finds the references to the name at pos, a
	// position in s, for ReferencesKey, on a goroutine of its own. Those in
	// the editor's own document (URI empty) are highlighted in the Theme's
	// Occurrence color until the document changes or Escape is pressed, and
	// all of them go to OnReferences at the end of Layout, for the app to
	// list.
	References   func(ctx context.Context, s EditorState, pos int) []Location
	OnReferences func([]Location)
	// TypeDefinitionKey, ImplementationKey and ReferencesKey run
	// TypeDefinition, Implementation and References at the cursor; they
	// start as the Default ones, and the zero Shortcut binds none.
	TypeDefinitionKey, ImplementationKey, ReferencesKey Shortcut
	// DefinitionHint is shown under a hover while Definition is set; it
	// starts as DefaultDefinitionHint, and empty shows none.
	DefinitionHint string
	// Complete, when set, offers completions: the editor asks it after one
	// of CompletionTriggers is typed, after two characters of a word typed
	// in a row, and on Ctrl+Space, and shows the items it answers in a
	// menu under the cursor, filtered by what is typed on. Up and Down
	// choose, Enter and Tab take the chosen item, Escape closes the menu.
	// It runs on a goroutine of its own, with ctx cancelled when its answer
	// is no longer wanted.
	Complete func(ctx context.Context, req CompletionRequest) CompletionResult
	// ResolveCompletion, when set, fills in a chosen item that has no
	// Documentation, on a goroutine of its own; the menu shows what it
	// gives back beside the item.
	ResolveCompletion func(ctx context.Context, item CompletionItem) CompletionItem
	// CompletionTriggers are the characters that ask Complete for
	// completions when typed, "." say.
	CompletionTriggers []string
	// SignatureHelp, when set, tells the signature of the call around pos,
	// a position in s, on a goroutine of its own: the editor asks it after
	// one of SignatureTriggers is typed, trigger, and then after every
	// transaction, trigger empty, while it answers; ok false, past the call
	// say, closes the popup it shows above the cursor, as Escape does.
	SignatureHelp func(ctx context.Context, s EditorState, pos int, trigger string) (sig Signature, ok bool)
	// Rename, when set, makes the edits that rename the symbol at pos, a
	// position in s, to name, on a goroutine of its own. F2 opens an input
	// over the name at the cursor, holding it; Enter renames to what is
	// typed there, Escape closes it. The edits to the editor's own document
	// (URI empty) are made as one transaction, if it has not changed since;
	// those to other documents go to OnWorkspaceEdit at the end of Layout.
	// An error shows at the symbol until the next transaction.
	Rename func(ctx context.Context, s EditorState, pos int, name string) ([]DocumentEdit, error)
	// CodeActions, when set, finds the actions on from, to, positions in s,
	// where diags are, on a goroutine of its own: the editor asks it when the
	// cursor reaches a line with a diagnostic (a bulb in the gutter shows
	// there are some) and on Ctrl+., which lists them under the cursor, as a
	// click on the bulb does. Up and Down choose, Enter takes the action,
	// Escape closes the list.
	CodeActions func(ctx context.Context, s EditorState, from, to int, diags []Diagnostic) []CodeAction
	// ApplyCodeAction makes the edits of an action taken, on a goroutine of
	// its own; they are made as Rename's are, an error shown at the cursor.
	ApplyCodeAction func(ctx context.Context, s EditorState, a CodeAction) ([]DocumentEdit, error)
	// OnWorkspaceEdit, when set, gets the edits a rename or a code action
	// makes to documents other than the editor's, for the app to apply.
	OnWorkspaceEdit func([]DocumentEdit)
	// SignatureTriggers are the characters that ask SignatureHelp when
	// typed, "(" and "," say.
	SignatureTriggers []string

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
	// shortcutDown is whether Ctrl, the shortcut modifier, is held, as
	// altDown is for Alt.
	shortcutDown bool

	// diags are the diagnostics, sorted, mapped through every change, and
	// jumped whether the last transaction was a jump to one of them, whose
	// message then shows at the cursor.
	diags  []Diagnostic
	jumped bool
	hover  hoverState
	// link is the name shown as a link, empty for none.
	link       SelectionRange
	definition *call[definitionAnswer]
	// references is a References call, occurrences the references it found
	// in the document, sorted, and foundRefs all of them, for OnReferences
	// after the frame.
	references  *call[[]Location]
	occurrences []SelectionRange
	foundRefs   []Location
	// elsewhere are definitions found in other documents, for OnDefinition
	// after the frame.
	elsewhere  []Location
	completion completion
	signature  signatureState
	rename     renameState
	actions    actionsState
	actionRowH int
	// diagsGen counts the SetDiagnostics calls, which ask for the code
	// actions again.
	diagsGen int
	// workspaceEdits are edits to other documents, for OnWorkspaceEdit
	// after the frame.
	workspaceEdits []DocumentEdit
	// snippet is the snippet whose fields Tab visits, nil for none.
	snippet *snippetSession
	popups  popupPointer
	// shaper lays out the popups' prose, and proseSizes are the runs of it
	// measured.
	shaper     *text.Shaper
	proseSizes map[proseKey]layout.Dimensions
	// menuRowH is the height of a completion's row as last drawn.
	menuRowH int
	// afterReport is what runs after the frame's updates are reported.
	afterReport []func()
	// posted is what Post queued for the next Layout.
	postMu sync.Mutex
	posted []func()
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
	e := &Editor{Theme: v.Theme, view: v, DefinitionHint: DefaultDefinitionHint,
		TypeDefinitionKey: DefaultTypeDefinitionKey, ImplementationKey: DefaultImplementationKey,
		ReferencesKey: DefaultReferencesKey}
	v.extraSpans = e.occurrenceSpans
	e.popups.over = -1
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
	e.diags, e.jumped, e.occurrences, e.snippet = nil, false, nil, nil
	e.clearHover()
	e.closeCompletion()
	e.closeSignature()
	e.rename.call.stop()
	e.rename = renameState{}
	e.actions.call.stop()
	e.actions = actionsState{}
	e.unshown = emptyChangeSet(len(s.doc))
	e.view.SetContent(s.content())
	return nil
}

// content is what the view shows of s. A final line end starts no line of a
// Content's Text, so one more is added: an empty last line stays a line.
func (s EditorState) content() Content {
	c := Content{Filename: s.conf.Filename, Text: s.doc + "\n", StyleTags: s.conf.StyleTags}
	c.Decorations, c.Widgets = s.lineDecorations()
	return c
}

// Post runs f on the goroutine that lays the editor out, at the start of
// its next Layout, where f may call any of the editor's methods. It may be
// called from any goroutine, a language client's say; the caller asks the
// window for that frame (app.Window.Invalidate).
func (e *Editor) Post(f func()) {
	e.postMu.Lock()
	e.posted = append(e.posted, f)
	e.postMu.Unlock()
}

func (e *Editor) runPosted() {
	e.postMu.Lock()
	fs := e.posted
	e.posted = nil
	e.postMu.Unlock()
	for _, f := range fs {
		f()
	}
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
	e.mapDiagnostics(t.Changes)
	e.mapSnippet(t)
	e.rename.err = ""
	if t.DocChanged() {
		e.rename.input = nil
		e.actions.open = false
	}
	if e.signature.open {
		e.signature.due = e.now.Add(completionDebounce)
	}
	e.jumped = false
	if c := &e.completion; c.open {
		c.since, c.pending = c.since.compose(t.Changes), c.pending.compose(t.Changes)
		c.from = t.Changes.MapPos(c.from, -1)
	}
	if t.DocChanged() {
		e.clearHover()
		e.occurrences = nil
	}
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
func (e *Editor) cellAt(offset int) pos { return e.cellIn(e.state, offset) }

// cellIn is cellAt of offset, a position in s.
func (e *Editor) cellIn(s EditorState, offset int) pos {
	l := s.LineAt(offset)
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
