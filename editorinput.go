package godemirror

import (
	"io"
	"strings"
	"unicode/utf8"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
)

// command is a CodeMirror command run on an editor: the transaction it
// makes, or ok false when it does not apply.
type command func(e *Editor) (TransactionSpec, bool)

// binding is a key of @codemirror/commands 6.11.1's standardKeymap, as it
// binds it off macOS: name with exactly mods held runs run, and with Shift
// held too, shift.
type binding struct {
	name       key.Name
	mods       key.Modifiers
	run, shift command
}

func stateCommand(f func(EditorState) (TransactionSpec, bool)) command {
	return func(e *Editor) (TransactionSpec, bool) { return f(e.state) }
}

var standardKeymap = []binding{
	{key.NameLeftArrow, 0, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.cursorByChar(false) }),
		stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.selectByChar(false) })},
	{key.NameLeftArrow, key.ModShortcut, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.cursorByGroup(false) }),
		stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.selectByGroup(false) })},
	{key.NameRightArrow, 0, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.cursorByChar(true) }),
		stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.selectByChar(true) })},
	{key.NameRightArrow, key.ModShortcut, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.cursorByGroup(true) }),
		stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.selectByGroup(true) })},
	{key.NameUpArrow, 0, func(e *Editor) (TransactionSpec, bool) { return e.cursorByLine(false) },
		func(e *Editor) (TransactionSpec, bool) { return e.selectByLine(false) }},
	{key.NameDownArrow, 0, func(e *Editor) (TransactionSpec, bool) { return e.cursorByLine(true) },
		func(e *Editor) (TransactionSpec, bool) { return e.selectByLine(true) }},
	{key.NamePageUp, 0, func(e *Editor) (TransactionSpec, bool) { return e.cursorByPage(false) },
		func(e *Editor) (TransactionSpec, bool) { return e.selectByPage(false) }},
	{key.NamePageDown, 0, func(e *Editor) (TransactionSpec, bool) { return e.cursorByPage(true) },
		func(e *Editor) (TransactionSpec, bool) { return e.selectByPage(true) }},
	{key.NameHome, 0, func(e *Editor) (TransactionSpec, bool) { return e.cursorLineBoundary(false) },
		func(e *Editor) (TransactionSpec, bool) { return e.selectLineBoundary(false) }},
	{key.NameHome, key.ModShortcut, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.cursorDoc(false) }),
		stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.selectDoc(false) })},
	{key.NameEnd, 0, func(e *Editor) (TransactionSpec, bool) { return e.cursorLineBoundary(true) },
		func(e *Editor) (TransactionSpec, bool) { return e.selectLineBoundary(true) }},
	{key.NameEnd, key.ModShortcut, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.cursorDoc(true) }),
		stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.selectDoc(true) })},
	{key.NameReturn, 0, stateCommand(EditorState.insertNewlineAndIndent), stateCommand(EditorState.insertNewlineAndIndent)},
	{key.NameEnter, 0, stateCommand(EditorState.insertNewlineAndIndent), stateCommand(EditorState.insertNewlineAndIndent)},
	{keySelectAll, key.ModShortcut, stateCommand(EditorState.selectAll), nil},
	{key.NameDeleteBackward, 0, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.deleteByChar(false, true) }),
		stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.deleteByChar(false, true) })},
	{key.NameDeleteForward, 0, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.deleteByChar(true, false) }), nil},
	{key.NameDeleteBackward, key.ModShortcut, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.deleteByGroup(false) }), nil},
	{key.NameDeleteForward, key.ModShortcut, stateCommand(func(s EditorState) (TransactionSpec, bool) { return s.deleteByGroup(true) }), nil},
}

// Clipboard keys, which a browser turns into the copy, cut and paste events
// CodeMirror handles.
const (
	keyCut   = "X"
	keyPaste = "V"
)

// keyFilters are the events the editor's view takes while it has focus.
func (e *Editor) keyFilters() []event.Filter {
	v := e.view
	filters := []event.Filter{
		key.FocusFilter{Target: v},
		transfer.TargetFilter{Target: v, Type: "application/text"},
		key.Filter{Focus: v, Name: keyCopy, Required: key.ModShortcut},
		key.Filter{Focus: v, Name: keyCut, Required: key.ModShortcut},
		key.Filter{Focus: v, Name: keyPaste, Required: key.ModShortcut},
	}
	for _, b := range standardKeymap {
		f := key.Filter{Focus: v, Name: b.name, Required: b.mods}
		if b.shift != nil {
			f.Optional = key.ModShift
		}
		filters = append(filters, f)
	}
	return filters
}

// handleInput takes the keys, text and clipboard contents addressed to the
// editor and dispatches what they do.
func (e *Editor) handleInput(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(e.keyFilters()...)
		if !ok {
			return
		}
		switch ev := ev.(type) {
		case key.FocusEvent:
			e.ime = imeState{}
		case key.Event:
			if ev.State == key.Press {
				e.onKey(gtx, ev)
			}
		case key.EditEvent:
			e.onEdit(ev)
		case key.SnippetEvent:
			e.sendSnippet(gtx, key.Range(ev))
		case key.SelectionEvent:
			e.onIMESelection(key.Range(ev))
		case key.CompositionEvent:
			e.ime.composing = ev.Start >= 0
		case transfer.DataEvent:
			e.onPaste(ev)
		}
	}
}

func (e *Editor) onKey(gtx layout.Context, ev key.Event) {
	mods := ev.Modifiers &^ key.ModShift
	shift := ev.Modifiers.Contain(key.ModShift)
	switch {
	case mods == key.ModShortcut && ev.Name == keyCopy:
		e.copy(gtx, false)
		return
	case mods == key.ModShortcut && ev.Name == keyCut:
		e.copy(gtx, true)
		return
	case mods == key.ModShortcut && ev.Name == keyPaste:
		gtx.Execute(clipboard.ReadCmd{Tag: e.view})
		return
	}
	for _, b := range standardKeymap {
		if b.name != ev.Name || b.mods != mods {
			continue
		}
		run := b.run
		if shift {
			run = b.shift
		}
		if run == nil {
			return
		}
		if spec, ok := run(e); ok {
			e.mustDispatch(spec)
		}
		return
	}
}

// mustDispatch dispatches a transaction a command made from the editor's own
// state, which is always valid.
func (e *Editor) mustDispatch(spec TransactionSpec) {
	if err := e.Dispatch(spec); err != nil {
		panic("godemirror: a command made an invalid transaction: " + err.Error())
	}
}

// copy writes what copiedRange takes to the clipboard and, for a cut,
// deletes it, as CodeMirror's copy and cut handlers do.
func (e *Editor) copy(gtx layout.Context, cut bool) {
	c := e.state.copiedRange()
	if c.text == "" && !c.linewise {
		return
	}
	if c.linewise {
		lastLinewiseCopy.Store(&c.text)
	} else {
		lastLinewiseCopy.Store(nil)
	}
	if cut {
		e.mustDispatch(TransactionSpec{Changes: c.ranges, ScrollIntoView: true, UserEvent: "delete.cut"})
	}
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(c.text))})
}

func (e *Editor) onPaste(ev transfer.DataEvent) {
	r := ev.Open()
	defer r.Close()
	content, err := io.ReadAll(r)
	// A clipboard that cannot be read, or holds no text, pastes nothing, as
	// a browser's paste event then carries no text.
	if err != nil || !utf8.Valid(content) {
		return
	}
	e.mustDispatch(e.state.doPaste(string(content), lastLinewiseCopy.Load()))
}

// imeState is what the editor last told Gio's text input of its document:
// the main range and the snippet around it, in runes of the whole document,
// as Gio counts them. composing is whether an input method is composing.
type imeState struct {
	sel       key.Range
	snippet   key.Snippet
	composing bool
}

// runeIndex is the number of runes of the document before pos.
func (e *Editor) runeIndex(pos int) int { return utf8.RuneCountInString(e.state.doc[:pos]) }

// offsetOfRune is the position of the rune at index i of the document,
// clamped to it.
func (e *Editor) offsetOfRune(i int) int {
	doc := e.state.doc
	for at := range doc {
		if i <= 0 {
			return at
		}
		i--
	}
	return len(doc)
}

// onEdit applies what the text input replaced, a typed character or an
// input method's text, as CodeMirror applies a DOM change.
func (e *Editor) onEdit(ev key.EditEvent) {
	from, to := e.offsetOfRune(min(ev.Range.Start, ev.Range.End)), e.offsetOfRune(max(ev.Range.Start, ev.Range.End))
	change := ChangeSpec{From: from, To: to, Insert: ev.Text}
	// Gio's text input puts the cursor after what it inserted.
	newSel := from + len(normalizeBreaks(ev.Text, e.state.conf.LineSeparator))
	e.mustDispatch(e.state.applyDefaultInsert(change, newSel, e.ime.composing))
}

// onIMESelection takes a selection an input method set as the main range.
func (e *Editor) onIMESelection(r key.Range) {
	main := e.state.sel.Main()
	if r == (key.Range{Start: e.runeIndex(main.Anchor()), End: e.runeIndex(main.Head())}) {
		return
	}
	sel := e.state.sel.ReplaceRange(Range(e.offsetOfRune(r.Start), e.offsetOfRune(r.End)), e.state.sel.main)
	e.mustDispatch(TransactionSpec{Selection: sel, UserEvent: "select"})
}

func (e *Editor) sendSnippet(gtx layout.Context, r key.Range) {
	from, to := e.offsetOfRune(min(r.Start, r.End)), e.offsetOfRune(max(r.Start, r.End))
	snippet := key.Snippet{Range: key.Range{Start: e.runeIndex(from), End: e.runeIndex(to)}, Text: e.state.doc[from:to]}
	if snippet != e.ime.snippet {
		e.ime.snippet = snippet
		gtx.Execute(key.SnippetCmd{Tag: e.view, Snippet: snippet})
	}
}

// tellIME tells Gio's text input where the main range is, when it moved.
func (e *Editor) tellIME(gtx layout.Context) {
	main := e.state.sel.Main()
	r := key.Range{Start: e.runeIndex(main.Anchor()), End: e.runeIndex(main.Head())}
	if r == e.ime.sel {
		return
	}
	e.ime.sel = r
	x, y := e.coordsAt(main.Head(), main.Assoc())
	caret := key.Caret{Ascent: float32(e.f.ch), Descent: 0}
	caret.Pos.X = float32(e.f.g.textX + x - e.view.scrollX())
	caret.Pos.Y = float32(y - e.view.scroll.Y + e.f.ch)
	gtx.Execute(key.SelectionCmd{Tag: e.view, Range: r, Caret: caret})
}
