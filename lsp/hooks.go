package lsp

import (
	"cmp"
	"context"
	"os"
	"slices"
	"strings"

	"github.com/lichdotdev/godemirror"
)

// kinds are the editor's completion kinds of LSP's.
var kinds = map[CompletionItemKind]godemirror.CompletionKind{
	KindMethod: godemirror.CompletionMethod, KindFunction: godemirror.CompletionFunction,
	KindConstructor: godemirror.CompletionFunction, KindField: godemirror.CompletionProperty,
	KindVariable: godemirror.CompletionVariable, KindClass: godemirror.CompletionClass,
	KindInterface: godemirror.CompletionInterface, KindModule: godemirror.CompletionNamespace,
	KindProperty: godemirror.CompletionProperty, KindEnum: godemirror.CompletionEnum,
	KindKeyword: godemirror.CompletionKeyword, KindEnumMember: godemirror.CompletionConstant,
	KindConstant: godemirror.CompletionConstant, KindStruct: godemirror.CompletionClass,
	KindTypeParameter: godemirror.CompletionType, KindText: godemirror.CompletionText,
	KindValue: godemirror.CompletionConstant, KindUnit: godemirror.CompletionConstant,
}

func (d *Document) complete(ctx context.Context, req godemirror.CompletionRequest) godemirror.CompletionResult {
	list, err := d.c.Completion(ctx, d.uri, d.position(req.State, req.Pos), req.Trigger)
	if err != nil {
		d.fail(err)
		return godemirror.CompletionResult{}
	}
	items := slices.Clone(list.Items)
	slices.SortStableFunc(items, func(a, b CompletionItem) int {
		return strings.Compare(cmp.Or(a.SortText, a.Label), cmp.Or(b.SortText, b.Label))
	})
	out := godemirror.CompletionResult{Incomplete: list.IsIncomplete, Items: make([]godemirror.CompletionItem, len(items))}
	for i, it := range items {
		out.Items[i] = d.completionItem(req, it)
	}
	return out
}

// completionItem is it as the editor offers it: its text edit's range, or
// the word being typed; its text with a snippet's placeholders taken out.
func (d *Document) completionItem(req godemirror.CompletionRequest, it CompletionItem) godemirror.CompletionItem {
	out := godemirror.CompletionItem{
		Label: it.Label, Kind: kinds[it.Kind], Detail: it.Detail, Documentation: markdown(it.Documentation),
		From: req.From, To: req.Pos, Insert: cmp.Or(it.InsertText, it.Label), Filter: it.FilterText, Data: it,
	}
	if it.TextEdit != nil {
		out.From, out.To = d.offset(req.State, it.TextEdit.Range.Start), d.offset(req.State, it.TextEdit.Range.End)
		out.Insert = it.TextEdit.NewText
	}
	out.Snippet = it.InsertTextFormat == Snippet
	return out
}

func (d *Document) resolve(ctx context.Context, item godemirror.CompletionItem) godemirror.CompletionItem {
	it, ok := item.Data.(CompletionItem)
	if !ok {
		return item
	}
	resolved, err := d.c.ResolveCompletion(ctx, it)
	if err != nil {
		d.fail(err)
		return item
	}
	item.Detail = cmp.Or(resolved.Detail, item.Detail)
	item.Documentation = markdown(resolved.Documentation)
	return item
}

func (d *Document) hover(ctx context.Context, s godemirror.EditorState, pos int) string {
	h, err := d.c.Hover(ctx, d.uri, d.position(s, pos))
	if err != nil || h == nil {
		d.fail(err)
		return ""
	}
	return markdown(h.Contents)
}

// goTo is an editor's GoTo that asks the server with find, and goes to the
// first place it answers.
func (d *Document) goTo(find func(context.Context, string, Position) ([]Location, error)) godemirror.GoTo {
	return func(ctx context.Context, s godemirror.EditorState, pos int) (godemirror.Location, bool) {
		locs, err := find(ctx, d.uri, d.position(s, pos))
		if err != nil || len(locs) == 0 {
			d.fail(err)
			return godemirror.Location{}, false
		}
		return d.location(s, locs[0], map[string][]string{}), true
	}
}

func (d *Document) references(ctx context.Context, s godemirror.EditorState, pos int) []godemirror.Location {
	locs, err := d.c.References(ctx, d.uri, d.position(s, pos), true)
	if err != nil {
		d.fail(err)
		return nil
	}
	files := map[string][]string{}
	out := make([]godemirror.Location, len(locs))
	for i, l := range locs {
		out[i] = d.location(s, l, files)
	}
	return out
}

// location is l as the editor takes it: in s, with an empty URI, when it is
// in the document; else in its own file, read once into files.
func (d *Document) location(s godemirror.EditorState, l Location, files map[string][]string) godemirror.Location {
	if l.URI == d.uri {
		start, end := d.offset(s, l.Range.Start), d.offset(s, l.Range.End)
		return godemirror.Location{Start: lineColumn(s, start), End: lineColumn(s, end)}
	}
	lines, ok := files[l.URI]
	if !ok {
		lines = d.fileLines(l.URI)
		files[l.URI] = lines
	}
	return godemirror.Location{URI: l.URI, Start: d.elsewhere(lines, l.Range.Start), End: d.elsewhere(lines, l.Range.End)}
}

func lineColumn(s godemirror.EditorState, pos int) godemirror.LineColumn {
	l := s.LineAt(pos)
	return godemirror.LineColumn{Line: l.Number - 1, Column: pos - l.From}
}

// fileLines are the lines of the file uri names, when the server counts
// columns in a unit other than UTF-8 bytes; nil when it does not, or the
// file cannot be read.
func (d *Document) fileLines(uri string) []string {
	if d.c.Encoding() == UTF8 {
		return nil
	}
	path, ok := FilePath(uri)
	if !ok {
		return nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(string(text), "\n")
}

// elsewhere is p, a position in a file of lines, with its column in UTF-8
// bytes: as the server gave it when there are no lines to read it in.
func (d *Document) elsewhere(lines []string, p Position) godemirror.LineColumn {
	out := godemirror.LineColumn{Line: p.Line, Column: p.Character}
	if p.Line < len(lines) {
		out.Column = d.c.Encoding().ByteColumn(strings.TrimSuffix(lines[p.Line], "\r"), p.Character)
	}
	return out
}

func (d *Document) signatureHelp(ctx context.Context, s godemirror.EditorState, pos int, trigger string) (godemirror.Signature, bool) {
	h, err := d.c.SignatureHelp(ctx, d.uri, d.position(s, pos), trigger)
	if err != nil || h == nil || len(h.Signatures) == 0 {
		d.fail(err)
		return godemirror.Signature{}, false
	}
	sig := h.Signatures[min(max(h.ActiveSignature, 0), len(h.Signatures)-1)]
	out := godemirror.Signature{Label: sig.Label, Doc: markdown(sig.Documentation), Active: -1}
	if active := cmp.Or(sig.ActiveParameter, h.ActiveParameter); active != nil {
		out.Active = *active
	}
	from := strings.IndexByte(sig.Label, '(') + 1
	for _, p := range sig.Parameters {
		r := godemirror.SignatureParam{From: d.c.Encoding().ByteColumn(sig.Label, p.Start), To: d.c.Encoding().ByteColumn(sig.Label, p.End)}
		if !p.Offsets {
			i := strings.Index(sig.Label[from:], p.Label)
			if i < 0 {
				out.Params = append(out.Params, godemirror.SignatureParam{})
				continue
			}
			r = godemirror.SignatureParam{From: from + i, To: from + i + len(p.Label)}
			from = r.To
		}
		out.Params = append(out.Params, r)
	}
	if out.Active >= 0 && out.Active < len(sig.Parameters) && sig.Parameters[out.Active].Documentation.Value != "" {
		out.Doc = strings.TrimSpace(markdown(sig.Parameters[out.Active].Documentation) + "\n\n" + out.Doc)
	}
	return out, true
}

func (d *Document) rename(ctx context.Context, s godemirror.EditorState, pos int, name string) ([]godemirror.DocumentEdit, error) {
	w, err := d.c.Rename(ctx, d.uri, d.position(s, pos), name)
	if err != nil {
		return nil, err
	}
	return d.workspaceEdit(s, w), nil
}

// workspaceEdit is w as the editor takes it: the document's edits read in
// s, with an empty URI; the others in their own files.
func (d *Document) workspaceEdit(s godemirror.EditorState, w WorkspaceEdit) []godemirror.DocumentEdit {
	return d.editorEdits(s, w.documentEdits())
}

func (d *Document) editorEdits(s godemirror.EditorState, edits []documentEdit) []godemirror.DocumentEdit {
	files := map[string][]string{}
	var out []godemirror.DocumentEdit
	for _, de := range edits {
		edit := godemirror.DocumentEdit{}
		if de.uri != d.uri {
			edit.URI = de.uri
		}
		for _, t := range de.edits {
			loc := d.location(s, Location{URI: de.uri, Range: t.Range}, files)
			edit.Edits = append(edit.Edits, godemirror.TextEdit{Start: loc.Start, End: loc.End, Text: t.NewText})
		}
		out = append(out, edit)
	}
	return out
}

func (d *Document) codeActions(ctx context.Context, s godemirror.EditorState, from, to int, diags []godemirror.Diagnostic) []godemirror.CodeAction {
	var lspDiags []Diagnostic
	for _, x := range diags {
		ld, ok := x.Data.(Diagnostic)
		if !ok {
			ld = Diagnostic{Severity: Severity(x.Severity), Message: x.Message, Source: x.Source, Code: Code(x.Code)}
		}
		ld.Range = Range{d.position(s, x.From), d.position(s, x.To)}
		lspDiags = append(lspDiags, ld)
	}
	actions, err := d.c.CodeActions(ctx, d.uri, Range{d.position(s, from), d.position(s, to)}, lspDiags)
	if err != nil {
		d.fail(err)
		return nil
	}
	out := make([]godemirror.CodeAction, len(actions))
	for i, a := range actions {
		out[i] = godemirror.CodeAction{Title: a.Title, Kind: a.Kind, Data: a}
	}
	return out
}

// applyCodeAction resolves the action when it has no edit, gives its edit
// to the editor, and runs its command, whose edits come back through
// workspace/applyEdit.
func (d *Document) applyCodeAction(ctx context.Context, s godemirror.EditorState, ga godemirror.CodeAction) ([]godemirror.DocumentEdit, error) {
	a, ok := ga.Data.(CodeAction)
	if !ok {
		return nil, nil
	}
	if a.Edit == nil {
		resolved, err := d.c.ResolveCodeAction(ctx, a)
		if err != nil {
			return nil, err
		}
		a = resolved
	}
	var edits []godemirror.DocumentEdit
	if a.Edit != nil {
		edits = d.workspaceEdit(s, *a.Edit)
	}
	if a.Command != nil {
		// The edit stands whatever the command does: its failure goes to
		// OnError, not over the edit.
		d.fail(d.c.ExecuteCommand(ctx, *a.Command))
	}
	return edits, nil
}
