package lsp

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/lichdotdev/godemirror"
)

// maxVersions is how many versions of a document a Document keeps for the
// server's diagnostics to be read in: diagnostics for an older one are
// dropped, the server publishing newer ones soon.
const maxVersions = 32

// Options are how Attach binds an Editor.
type Options struct {
	// LanguageID is the document's language as LSP names it ("go",
	// "typescript", "python"); Attach fails without it.
	LanguageID string
	// Invalidate asks the window for a frame (app.Window.Invalidate):
	// diagnostics come from the server between frames, and the editor shows
	// them on the next one. It is called from the goroutine that reads from
	// the server.
	Invalidate func()
	// OnError, when set, hears of every request the server failed or a
	// change the client could not send, from the goroutine that met it: the
	// UI's for a change, a request's own for a request. Without it they are
	// dropped: the editor shows nothing for a request that failed.
	OnError func(error)
}

// Document is an Editor bound to a document of a server by Attach.
type Document struct {
	c    *Client
	e    *godemirror.Editor
	uri  string
	opts Options

	// version is the document's version the server last heard of, and
	// states the states the server may still publish diagnostics for, by
	// version, oldest first: each with the changes from it to the next.
	version int
	states  []versioned
	// onUpdate is the editor's OnUpdate before Attach, still called.
	onUpdate func(godemirror.Update)
	stop     func()

	mu sync.Mutex
	// published is the last diagnostics the server sent, not yet shown.
	published *PublishDiagnostics
}

type versioned struct {
	version int
	state   godemirror.EditorState
	changes godemirror.ChangeSet
}

// Attach opens the editor's document on the server, as uri, and binds the
// two: every edit goes to the server (incrementally when it accepts that,
// whole otherwise), the server's diagnostics show in the editor, and the
// editor's hooks (Complete, ResolveCompletion, CompletionTriggers, Hover,
// Definition, TypeDefinition, Implementation, References, SignatureHelp,
// Rename) ask the server,
// for what it offers. The editor's OnUpdate, if
// set, is still called after the server hears of each update. Attach runs
// on the goroutine that lays the editor out; Close undoes it.
func Attach(c *Client, e *godemirror.Editor, uri string, opts Options) (*Document, error) {
	if opts.LanguageID == "" {
		return nil, errors.New("lsp: Attach needs a LanguageID")
	}
	s := e.State()
	d := &Document{c: c, e: e, uri: uri, opts: opts, version: 1, onUpdate: e.OnUpdate}
	d.states = []versioned{{version: 1, state: s}}
	if err := c.DidOpen(uri, opts.LanguageID, d.version, s.Doc()); err != nil {
		return nil, err
	}
	d.stop = c.WatchDiagnostics(uri, d.onPublish)
	e.OnUpdate = d.update
	if c.HasCompletion() {
		e.Complete, e.ResolveCompletion, e.CompletionTriggers = d.complete, d.resolve, c.CompletionTriggers()
	}
	if c.HasHover() {
		e.Hover = d.hover
	}
	if c.HasDefinition() {
		e.Definition = d.goTo(c.Definition)
	}
	if c.HasTypeDefinition() {
		e.TypeDefinition = d.goTo(c.TypeDefinition)
	}
	if c.HasImplementation() {
		e.Implementation = d.goTo(c.Implementation)
	}
	if c.HasReferences() {
		e.References = d.references
	}
	if c.HasRename() {
		e.Rename = d.rename
	}
	if c.HasSignatureHelp() {
		e.SignatureHelp, e.SignatureTriggers = d.signatureHelp, c.SignatureTriggers()
	}
	return d, nil
}

// Save tells the server the document was saved, as it now is.
func (d *Document) Save() error {
	_, withText := d.c.Sync()
	if !withText {
		return d.c.DidSave(d.uri, nil)
	}
	text := d.e.State().Doc()
	return d.c.DidSave(d.uri, &text)
}

// Close closes the document on the server, and gives the editor back its
// OnUpdate and no hooks, with its diagnostics cleared. It runs on the
// goroutine that lays the editor out.
func (d *Document) Close() error {
	d.stop()
	e := d.e
	e.OnUpdate = d.onUpdate
	e.Complete, e.ResolveCompletion, e.CompletionTriggers = nil, nil, nil
	e.Hover, e.Definition, e.TypeDefinition, e.Implementation, e.References = nil, nil, nil, nil, nil
	e.SignatureHelp, e.SignatureTriggers, e.Rename = nil, nil, nil
	e.SetDiagnostics(nil)
	return d.c.DidClose(d.uri)
}

func (d *Document) fail(err error) {
	if err != nil && !errors.Is(err, context.Canceled) && d.opts.OnError != nil {
		d.opts.OnError(err)
	}
}

// update sends an editor update to the server as the next version.
func (d *Document) update(u godemirror.Update) {
	if u.DocChanged() {
		last := &d.states[len(d.states)-1]
		last.changes = u.Changes
		d.version++
		d.states = append(d.states, versioned{version: d.version, state: u.State})
		if len(d.states) > maxVersions {
			d.states = slices.Delete(d.states, 0, len(d.states)-maxVersions)
		}
		d.fail(d.c.DidChange(d.uri, d.version, d.contentChanges(u)))
	}
	if d.onUpdate != nil {
		d.onUpdate(u)
	}
}

// contentChanges are u's changes as the server takes them: each changed
// range, last first so that each is where the one before it left it, or
// the whole text.
func (d *Document) contentChanges(u godemirror.Update) []ContentChange {
	if kind, _ := d.c.Sync(); kind != SyncIncremental {
		return []ContentChange{{Text: u.State.Doc()}}
	}
	var out []ContentChange
	u.Changes.IterChanges(func(fromA, toA, _, _ int, text string) {
		r := Range{d.position(u.StartState, fromA), d.position(u.StartState, toA)}
		out = append(out, ContentChange{Range: &r, Text: text})
	})
	slices.Reverse(out)
	return out
}

// position is pos, a position in s, as the server counts it.
func (d *Document) position(s godemirror.EditorState, pos int) Position {
	l := s.LineAt(pos)
	return Position{Line: l.Number - 1, Character: d.c.Encoding().Character(l.Text, pos-l.From)}
}

// offset is p, a position the server gave in s, as a position in s.
func (d *Document) offset(s godemirror.EditorState, p Position) int {
	line := s.Line(min(max(p.Line+1, 1), s.Lines()))
	if p.Line >= s.Lines() {
		return len(s.Doc())
	}
	return line.From + d.c.Encoding().ByteColumn(line.Text, p.Character)
}

func (d *Document) onPublish(p PublishDiagnostics) {
	d.mu.Lock()
	d.published = &p
	d.mu.Unlock()
	d.e.Post(d.showDiagnostics)
	if d.opts.Invalidate != nil {
		d.opts.Invalidate()
	}
}

// showDiagnostics gives the editor the last diagnostics published, read in
// the version they were published for and mapped through the edits since.
func (d *Document) showDiagnostics() {
	d.mu.Lock()
	p := d.published
	d.published = nil
	d.mu.Unlock()
	if p == nil {
		return
	}
	at := len(d.states) - 1
	if p.Version != nil {
		i := slices.IndexFunc(d.states, func(v versioned) bool { return v.version == *p.Version })
		if i < 0 {
			return
		}
		at = i
	}
	d.states = d.states[at:]
	s := d.states[0].state
	out := make([]godemirror.Diagnostic, 0, len(p.Diagnostics))
	for _, x := range p.Diagnostics {
		from, to := d.offset(s, x.Range.Start), d.offset(s, x.Range.End)
		for _, v := range d.states[:len(d.states)-1] {
			from, to = v.changes.MapPos(from, 1), v.changes.MapPos(to, -1)
		}
		out = append(out, godemirror.Diagnostic{
			From: from, To: max(to, from), Severity: godemirror.Severity(cmp.Or(x.Severity, SeverityError)),
			Message: x.Message, Source: x.Source, Code: string(x.Code),
		})
	}
	d.e.SetDiagnostics(out)
}
