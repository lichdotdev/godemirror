package godemirror

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"gioui.org/io/key"
	"gioui.org/layout"
)

// completionMinWord is how many characters of a word typed in a row open the
// completions unasked.
const completionMinWord = 2

// CompletionKind is what a completion names, which picks the badge before
// its label: one of the kinds below, those of @codemirror/autocomplete's
// Completion.type, or "" for none.
type CompletionKind string

// The kinds of completion.
const (
	CompletionClass     CompletionKind = "class"
	CompletionConstant  CompletionKind = "constant"
	CompletionEnum      CompletionKind = "enum"
	CompletionFunction  CompletionKind = "function"
	CompletionInterface CompletionKind = "interface"
	CompletionKeyword   CompletionKind = "keyword"
	CompletionMethod    CompletionKind = "method"
	CompletionNamespace CompletionKind = "namespace"
	CompletionProperty  CompletionKind = "property"
	CompletionText      CompletionKind = "text"
	CompletionType      CompletionKind = "type"
	CompletionVariable  CompletionKind = "variable"
)

// CompletionItem is one completion an Editor offers.
type CompletionItem struct {
	Label string
	Kind  CompletionKind
	// Detail, a type or a signature, shows at the right of the menu and
	// over Documentation beside it, while the item is chosen.
	Detail, Documentation string
	// From and To are the range Insert replaces, positions in the state
	// the completions were asked in; To grows to the cursor as the user
	// types on. A hook with nothing better takes CompletionRequest.From and
	// Pos.
	From, To int
	// Insert replaces the range when the item is taken; empty inserts
	// Label.
	Insert string
	// Filter is what the text typed is matched against; empty matches
	// Label.
	Filter string
	// Data is the hook's own, for ResolveCompletion to read.
	Data any
}

// CompletionRequest is what Complete is asked: the completions at Pos, in
// State, where the word being typed starts at From. Trigger is the trigger
// character just typed (CompletionTriggers), empty when the user asked with
// Ctrl+Space (Explicit) or typed a word.
type CompletionRequest struct {
	State    EditorState
	Pos      int
	From     int
	Trigger  string
	Explicit bool
}

// CompletionResult is what Complete answers. Incomplete asks Complete again
// as the user types on, instead of filtering Items.
type CompletionResult struct {
	Items      []CompletionItem
	Incomplete bool
}

// completion is the completions offered, or being asked for.
type completion struct {
	open bool
	// from is where the word being completed starts, and since the
	// changes made after the items were asked for.
	from  int
	since ChangeSet
	items []CompletionItem
	// incomplete is the result's Incomplete, and asked the text the items
	// were asked for.
	incomplete bool
	asked      string
	// filtered is the text typed the shown items were matched against.
	filtered string
	// shown are the indices of the items the text typed matches, sel the
	// chosen one among them and top the first on screen.
	shown []int
	sel   int
	top   int
	call  *call[CompletionResult]
	// resolve is a ResolveCompletion call for the item at resolving, and
	// resolved the items asked about, each asked once.
	resolve   *call[CompletionItem]
	resolving int
	resolved  []int
}

// wordBefore is where the word before pos starts, and how many characters
// long it is.
func (s EditorState) wordBefore(pos int) (from, chars int) {
	line := s.LineAt(pos)
	from = pos
	for from > line.From {
		r, n := utf8.DecodeLastRuneInString(s.doc[line.From:from])
		if categorize(string(r)) != categoryWord {
			break
		}
		from -= n
		chars++
	}
	return from, chars
}

// afterTyping opens, filters or closes the completions after text was
// typed.
func (e *Editor) afterTyping(text string) {
	if e.Complete == nil || len(e.state.sel.ranges) != 1 {
		return
	}
	head := e.state.sel.Main().Head()
	last, _ := utf8.DecodeLastRuneInString(text)
	switch {
	case slices.Contains(e.CompletionTriggers, string(last)):
		e.askCompletion(string(last), false)
	case e.completion.open && categorize(string(last)) == categoryWord:
	case e.completion.open:
		e.closeCompletion()
	case categorize(string(last)) == categoryWord:
		if _, n := e.state.wordBefore(head); n >= completionMinWord {
			e.askCompletion("", false)
		}
	}
}

// askCompletion asks Complete for the completions at the cursor, after the
// frame's updates are reported, so that a language server hears of the
// edits before the request.
func (e *Editor) askCompletion(trigger string, explicit bool) {
	hook := e.Complete
	if hook == nil {
		return
	}
	c := &e.completion
	c.call.stop()
	head := e.state.sel.Main().Head()
	from, _ := e.state.wordBefore(head)
	typed := e.state.doc[from:head]
	*c = completion{open: true, from: from, since: emptyChangeSet(len(e.state.doc)), asked: typed, filtered: typed}
	req := CompletionRequest{State: e.state, Pos: head, From: from, Trigger: trigger, Explicit: explicit}
	e.later(func() {
		// Closed, or asked again, since: this answer is not wanted.
		if !c.open || c.from != from || c.call != nil {
			return
		}
		c.call = startCall(func(ctx context.Context) CompletionResult { return hook(ctx, req) })
	})
}

func (e *Editor) closeCompletion() {
	e.completion.call.stop()
	e.completion.resolve.stop()
	e.completion = completion{}
}

// typedSince is the text typed from the word's start to the cursor, ok
// false when the cursor left the word.
func (e *Editor) typedSince() (string, bool) {
	c, main := &e.completion, e.state.sel.Main()
	head := main.Head()
	if !main.Empty() || len(e.state.sel.ranges) != 1 || head < c.from {
		return "", false
	}
	if from, _ := e.state.wordBefore(head); from > c.from {
		return "", false
	}
	return e.state.doc[c.from:head], true
}

// refilterCompletion matches the items against what is typed now, or asks
// again when the result was incomplete.
func (e *Editor) refilterCompletion() {
	c := &e.completion
	typed, ok := e.typedSince()
	if !ok {
		e.closeCompletion()
		return
	}
	if c.incomplete && typed != c.asked {
		e.askCompletion("", false)
		return
	}
	c.filtered = typed
	c.shown = c.shown[:0]
	for i, it := range c.items {
		if matches(cmpOr(it.Filter, it.Label), typed) {
			c.shown = append(c.shown, i)
		}
	}
	if c.call == nil && len(c.shown) == 0 {
		e.closeCompletion()
		return
	}
	c.sel, c.top = 0, 0
	e.resolveChosen()
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// matches reports whether typed's characters appear in label in order,
// case aside, as a fuzzy match.
func matches(label, typed string) bool {
	label = strings.ToLower(label)
	for _, r := range strings.ToLower(typed) {
		i := strings.IndexRune(label, r)
		if i < 0 {
			return false
		}
		label = label[i+utf8.RuneLen(r):]
	}
	return true
}

// takeCompletions takes Complete's and ResolveCompletion's answers when
// they come, and closes the completions when the cursor leaves the word.
func (e *Editor) takeCompletions(gtx layout.Context) {
	c := &e.completion
	if !c.open {
		return
	}
	typed, ok := e.typedSince()
	if !ok {
		e.closeCompletion()
		return
	}
	if c.call == nil && typed != c.filtered {
		e.refilterCompletion()
	}
	if c.call != nil {
		if r, ok := c.call.poll(gtx); ok {
			c.call, c.items, c.incomplete = nil, r.Items, r.Incomplete
			e.refilterCompletion()
		}
	}
	if c.resolve != nil {
		if it, ok := c.resolve.poll(gtx); ok {
			c.resolve = nil
			c.items[c.resolving] = it
		}
	}
}

// resolveChosen asks ResolveCompletion to fill in the chosen item, when it
// has no documentation yet.
func (e *Editor) resolveChosen() {
	c, hook := &e.completion, e.ResolveCompletion
	if hook == nil || len(c.shown) == 0 {
		return
	}
	i := c.shown[c.sel]
	if c.items[i].Documentation != "" || slices.Contains(c.resolved, i) {
		return
	}
	if c.resolve != nil {
		c.resolve.stop()
		c.resolved = slices.DeleteFunc(c.resolved, func(j int) bool { return j == c.resolving })
	}
	c.resolve = nil
	it := c.items[i]
	c.resolving, c.resolved = i, append(c.resolved, i)
	e.later(func() {
		if !c.open || c.resolving != i || c.resolve != nil {
			return
		}
		c.resolve = startCall(func(ctx context.Context) CompletionItem { return hook(ctx, it) })
	})
}

// completionKey runs a key on the open completions, and reports whether
// it took it: arrows and pages choose, Enter and Tab take the chosen item,
// Escape closes.
func (e *Editor) completionKey(ev key.Event) bool {
	c := &e.completion
	if !c.open || len(c.shown) == 0 || ev.Modifiers != 0 {
		return false
	}
	switch ev.Name {
	case key.NameUpArrow:
		e.chooseCompletion(c.sel - 1)
	case key.NameDownArrow:
		e.chooseCompletion(c.sel + 1)
	case key.NamePageUp:
		e.chooseCompletion(max(c.sel-completionRows, 0))
	case key.NamePageDown:
		e.chooseCompletion(min(c.sel+completionRows, len(c.shown)-1))
	case key.NameReturn, key.NameEnter, key.NameTab:
		e.acceptCompletion()
	case key.NameEscape:
		e.closeCompletion()
	default:
		return false
	}
	return true
}

// chooseCompletion chooses the shown item at i, wrapping around the ends.
func (e *Editor) chooseCompletion(i int) {
	c := &e.completion
	n := len(c.shown)
	c.sel = (i%n + n) % n
	if c.sel < c.top {
		c.top = c.sel
	} else if c.sel >= c.top+completionRows {
		c.top = c.sel - completionRows + 1
	}
	e.resolveChosen()
}

// acceptCompletion replaces the chosen item's range, grown to the cursor,
// with its text. A range outside the document is clamped to it.
func (e *Editor) acceptCompletion() {
	c := &e.completion
	it := c.items[c.shown[c.sel]]
	head := e.state.sel.Main().Head()
	asked := c.since.Len()
	from := e.clampPos(c.since.MapPos(min(max(it.From, 0), asked), -1))
	to := max(e.clampPos(c.since.MapPos(min(max(it.To, 0), asked), 1)), head)
	from = min(from, head)
	insert := cmpOr(it.Insert, it.Label)
	e.closeCompletion()
	end := from + len(insert)
	e.mustDispatch(TransactionSpec{
		Changes:   []ChangeSpec{{From: from, To: to, Insert: insert}},
		Selection: SingleSelection(end, end), ScrollIntoView: true, UserEvent: "input.complete",
	})
}

// later runs f after the frame's updates are reported.
func (e *Editor) later(f func()) { e.afterReport = append(e.afterReport, f) }

func (e *Editor) runLater() {
	fs := e.afterReport
	e.afterReport = nil
	for _, f := range fs {
		f()
	}
}
