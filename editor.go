package godemirror

import (
	"slices"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/glyphs"
)

// Editor is a View whose document can be changed, CodeMirror's EditorView
// over an EditorState: a caller changes it with Dispatch and hears of every
// change through OnUpdate. Its zero value is not usable; build one with
// NewEditor or NewEditorFromFonts.
//
// Like a View, an Editor belongs to the goroutine that lays it out.
type Editor struct {
	// Wrap breaks lines at the editor's width instead of scrolling sideways.
	Wrap bool
	// Theme colors the editor; it starts as DarkTheme.
	Theme Theme
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
	t, err := e.state.update(spec)
	if err != nil {
		return err
	}
	e.state = t.State
	e.unshown = e.unshown.compose(t.Changes)
	e.since = append(e.since, t)
	return nil
}

// Layout draws the editor into the whole of gtx.Constraints.Max, as
// View.Layout draws a view, takes the pointer selection into its state, and
// then reports what was dispatched to OnUpdate. The errors are View.Layout's.
func (e *Editor) Layout(gtx layout.Context) (layout.Dimensions, error) {
	e.view.Wrap, e.view.Theme = e.Wrap, e.Theme
	e.show()
	dims, err := e.view.Layout(gtx)
	if err != nil {
		return dims, err
	}
	e.takeSelection()
	e.report(gtx)
	return dims, nil
}

// show gives the view what changed since it last showed the state.
func (e *Editor) show() {
	edited := !e.unshown.Empty()
	if edited {
		fromA, toA := len(e.shown.doc), 0
		e.unshown.IterChanges(func(a, b, _, _ int, _ string) {
			fromA, toA = min(fromA, a), max(toA, b)
		})
		keepTop := e.shown.LineAt(fromA).Number - 1
		keepBottom := e.shown.Lines() - e.shown.LineAt(toA).Number
		e.view.showEdit(e.state.content(), keepTop, keepBottom)
		e.unshown = emptyChangeSet(len(e.state.doc))
	}
	if !edited && e.mark != nil && e.state.sel.equal(e.shown.sel) {
		return
	}
	e.shown = e.state
	ranges := e.state.sel.ranges
	e.mark = make([]selection, len(ranges))
	for i, r := range ranges {
		e.mark[i] = selection{e.cellAt(r.Anchor()), e.cellAt(r.Head())}
	}
	e.view.sel = e.mark[e.state.sel.main]
	e.view.others = nil
	for i, s := range e.mark {
		if i != e.state.sel.main {
			e.view.others = append(e.view.others, s)
		}
	}
}

// takeSelection dispatches the selection the pointer or a key made on the
// view, if it made one.
func (e *Editor) takeSelection() {
	v := e.view
	if v.sel == e.mark[e.state.sel.main] && len(v.others) == len(e.mark)-1 {
		return
	}
	ranges := []SelectionRange{Range(e.offsetAt(v.sel.anchor), e.offsetAt(v.sel.head))}
	for _, s := range v.others {
		ranges = append(ranges, Range(e.offsetAt(s.anchor), e.offsetAt(s.head)))
	}
	err := e.Dispatch(TransactionSpec{Selection: NewSelection(ranges, 0), UserEvent: "select"})
	if err != nil {
		panic("godemirror: the view's selection is outside the editor's document: " + err.Error())
	}
	e.show()
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

// equal reports whether s and o hold the same ranges, in direction and
// stickiness too, and the same main range.
func (s EditorSelection) equal(o EditorSelection) bool {
	return s.main == o.main && slices.Equal(s.ranges, o.ranges)
}
