package lsp

import (
	"context"
	"encoding/json"
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
)

// editorHarness lays an Editor out frame by frame through Gio's own input
// router, as an app's window would.
type editorHarness struct {
	t      *testing.T
	e      *godemirror.Editor
	ops    op.Ops
	router input.Router
	now    time.Time
}

func newEditorHarness(t *testing.T, doc string) *editorHarness {
	t.Helper()
	e, err := godemirror.NewEditor("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetState(godemirror.EditorStateConfig{Doc: doc, Filename: "a.go"}); err != nil {
		t.Fatal(err)
	}
	h := &editorHarness{t: t, e: e, now: time.Unix(1, 0)}
	h.frame()
	return h
}

func (h *editorHarness) frame() {
	h.t.Helper()
	h.ops.Reset()
	gtx := layout.Context{Ops: &h.ops, Now: h.now, Source: h.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 200))}
	if _, err := h.e.Layout(gtx); err != nil {
		h.t.Fatal(err)
	}
	h.router.Frame(&h.ops)
}

// until lays the editor out, a frame a millisecond, until cond holds.
func (h *editorHarness) until(what string, cond func() bool) {
	h.t.Helper()
	for range 5000 {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
		h.now = h.now.Add(time.Millisecond)
		h.frame()
	}
	h.t.Fatalf("never %s", what)
}

func (h *editorHarness) dispatch(spec godemirror.TransactionSpec) {
	h.t.Helper()
	if err := h.e.Dispatch(spec); err != nil {
		h.t.Fatal(err)
	}
	h.frame()
}

func attached(t *testing.T, doc string, caps map[string]any) (*editorHarness, *fakeServer, *Document) {
	t.Helper()
	s := newFakeServer(t, caps)
	c := s.connect(Config{})
	h := newEditorHarness(t, doc)
	d, err := Attach(c, h.e, "file:///a.go", Options{LanguageID: "go", OnError: func(err error) { t.Error(err) }})
	if err != nil {
		t.Fatal(err)
	}
	s.wait("textDocument/didOpen")
	return h, s, d
}

type didChange struct {
	TextDocument   versionedDocument `json:"textDocument"`
	ContentChanges []ContentChange   `json:"contentChanges"`
}

func TestAttachOpensTheDocument(t *testing.T) {
	_, s, _ := attached(t, "package a", map[string]any{})
	p := unmarshal[map[string]map[string]any](t, s.wait("textDocument/didOpen"))["textDocument"]
	if p["uri"] != "file:///a.go" || p["languageId"] != "go" || p["text"] != "package a" || p["version"] != float64(1) {
		t.Fatalf("didOpen %v", p)
	}
}

func TestEditsGoToAnIncrementalServerInUTF16(t *testing.T) {
	h, s, _ := attached(t, "é😀x\nab", map[string]any{"textDocumentSync": 2})
	h.dispatch(godemirror.TransactionSpec{Changes: []godemirror.ChangeSpec{
		{From: 6, To: 7, Insert: "Y"},
		{From: 8, To: 9, Insert: "zz"},
	}})
	got := unmarshal[didChange](t, s.wait("textDocument/didChange"))
	if got.TextDocument.Version != 2 || len(got.ContentChanges) != 2 {
		t.Fatalf("didChange %+v", got)
	}
	last, first := got.ContentChanges[0], got.ContentChanges[1]
	if last.Range.Start != (Position{1, 0}) || last.Range.End != (Position{1, 1}) || last.Text != "zz" {
		t.Fatalf("the later change went first as %+v", last)
	}
	if first.Range.Start != (Position{0, 3}) || first.Range.End != (Position{0, 4}) || first.Text != "Y" {
		t.Fatalf("x is at UTF-16 3..4, sent %+v", first)
	}
}

func TestEditsGoWholeToAFullSyncServer(t *testing.T) {
	h, s, _ := attached(t, "ab", map[string]any{"textDocumentSync": 1})
	h.dispatch(godemirror.TransactionSpec{Changes: []godemirror.ChangeSpec{{From: 1, To: 1, Insert: "X"}}})
	got := unmarshal[didChange](t, s.wait("textDocument/didChange"))
	if len(got.ContentChanges) != 1 || got.ContentChanges[0].Range != nil || got.ContentChanges[0].Text != "aXb" {
		t.Fatalf("didChange %+v", got)
	}
}

func TestDiagnosticsShowMappedThroughLaterEdits(t *testing.T) {
	h, s, _ := attached(t, "x := unblock()", map[string]any{"textDocumentSync": 2})
	h.dispatch(godemirror.TransactionSpec{Changes: []godemirror.ChangeSpec{{From: 0, Insert: "// c\n"}}})
	s.wait("textDocument/didChange")
	v := 1
	s.publish(PublishDiagnostics{URI: "file:///a.go", Version: &v, Diagnostics: []Diagnostic{{
		Range: Range{Position{0, 5}, Position{0, 12}}, Message: "undefined: unblock", Source: "compiler", Code: "UndeclaredName",
	}}})
	h.until("shown", func() bool { return len(h.e.Diagnostics()) == 1 })
	d := h.e.Diagnostics()[0]
	if d.From != 10 || d.To != 17 || d.Severity != godemirror.SeverityError || d.Code != "UndeclaredName" {
		t.Fatalf("diagnostic %+v, want unblock at 10..17 in the edited document", d)
	}
	v = 7
	s.publish(PublishDiagnostics{URI: "file:///a.go", Version: &v, Diagnostics: []Diagnostic{{Message: "stale"}}})
	s.publish(PublishDiagnostics{URI: "file:///a.go", Diagnostics: nil})
	h.until("cleared", func() bool { return len(h.e.Diagnostics()) == 0 })
}

func TestCompletionAsksTheServer(t *testing.T) {
	h, s, _ := attached(t, "fmt.Pr", map[string]any{
		"positionEncoding":   "utf-8",
		"completionProvider": map[string]any{"triggerCharacters": []string{"."}, "resolveProvider": true},
	})
	s.handlers["textDocument/completion"] = func(json.RawMessage) any {
		return json.RawMessage(`{"items":[
			{"label":"Printf","kind":3,"sortText":"b","insertTextFormat":2,"textEdit":{"range":{"start":{"line":0,"character":4},"end":{"line":0,"character":6}},"newText":"Printf(${1:format}, ${2:a})$0"}},
			{"label":"Println","kind":3,"sortText":"a","detail":"func(a ...any)","data":{"id":7}}]}`)
	}
	s.handlers["completionItem/resolve"] = func(p json.RawMessage) any {
		it := unmarshal[CompletionItem](t, p)
		it.Documentation = Markup{Kind: "markdown", Value: "Println **formats**."}
		return it
	}
	if got := h.e.CompletionTriggers; len(got) != 1 || got[0] != "." {
		t.Fatalf("triggers %v", got)
	}
	ctx := context.Background()
	st := h.e.State()
	r := h.e.Complete(ctx, godemirror.CompletionRequest{State: st, Pos: 6, From: 4})
	if len(r.Items) != 2 || r.Items[0].Label != "Println" {
		t.Fatalf("items %+v, want Println first by sortText", r.Items)
	}
	pf := r.Items[1]
	if pf.From != 4 || pf.To != 6 || pf.Insert != "Printf(${1:format}, ${2:a})$0" || !pf.Snippet || pf.Kind != godemirror.CompletionFunction {
		t.Fatalf("Printf as %+v", pf)
	}
	pl := h.e.ResolveCompletion(ctx, r.Items[0])
	if pl.Documentation != "Println **formats**." {
		t.Fatalf("resolved docs %q", pl.Documentation)
	}
	p := unmarshal[positionParams](t, s.wait("textDocument/completion"))
	if p.Position != (Position{0, 6}) {
		t.Fatalf("asked at %+v", p.Position)
	}
}

func TestHoverHandsTheServersMarkdownOver(t *testing.T) {
	h, s, _ := attached(t, "x", map[string]any{"hoverProvider": true})
	md := "```go\nfunc vimBufOf(gen int) int\n```\n\nvimBufOf is the buffer."
	s.handlers["textDocument/hover"] = func(json.RawMessage) any {
		return map[string]any{"contents": map[string]string{"kind": "markdown", "value": md}}
	}
	if got := h.e.Hover(context.Background(), h.e.State(), 0); got != md {
		t.Fatalf("hover %q, want the server's markdown", got)
	}
}

func TestDefinitionHereOrElsewhere(t *testing.T) {
	h, s, _ := attached(t, "é := 1\né", map[string]any{"definitionProvider": true})
	s.handlers["textDocument/definition"] = func(json.RawMessage) any {
		return []Location{{URI: "file:///a.go", Range: Range{Position{0, 0}, Position{0, 1}}}}
	}
	loc, ok := h.e.Definition(context.Background(), h.e.State(), 9)
	if !ok || loc.URI != "" || loc.Start != (godemirror.LineColumn{}) || loc.End != (godemirror.LineColumn{Line: 0, Column: 2}) {
		t.Fatalf("here: %+v %v, want 0:0..0:2 in bytes", loc, ok)
	}
	s.handlers["textDocument/definition"] = func(json.RawMessage) any {
		return []Location{{URI: "file:///nowhere/b.go", Range: Range{Position{4, 2}, Position{4, 5}}}}
	}
	loc, ok = h.e.Definition(context.Background(), h.e.State(), 9)
	if !ok || loc.URI != "file:///nowhere/b.go" || loc.Start != (godemirror.LineColumn{Line: 4, Column: 2}) {
		t.Fatalf("elsewhere: %+v %v", loc, ok)
	}
}

func TestCloseGivesTheEditorBack(t *testing.T) {
	h, s, d := attached(t, "x", map[string]any{"hoverProvider": true, "definitionProvider": true, "completionProvider": map[string]any{}})
	h.e.SetDiagnostics([]godemirror.Diagnostic{{From: 0, To: 1}})
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	s.wait("textDocument/didClose")
	if h.e.Hover != nil || h.e.Definition != nil || h.e.Complete != nil || h.e.OnUpdate != nil || len(h.e.Diagnostics()) != 0 {
		t.Fatal("Close left the editor bound")
	}
}

func TestAttachNeedsALanguage(t *testing.T) {
	c := newFakeServer(t, map[string]any{}).connect(Config{})
	if _, err := Attach(c, newEditorHarness(t, "").e, "file:///a", Options{}); err == nil {
		t.Fatal("attached with no LanguageID")
	}
}

func TestTheOtherGoTosAndReferencesAskTheServer(t *testing.T) {
	h, s, _ := attached(t, "var x T", map[string]any{
		"typeDefinitionProvider": true, "implementationProvider": true, "referencesProvider": true,
	})
	r := Range{Position{0, 4}, Position{0, 5}}
	for _, m := range []string{"textDocument/typeDefinition", "textDocument/implementation", "textDocument/references"} {
		s.handlers[m] = func(json.RawMessage) any {
			return []Location{{URI: "file:///a.go", Range: r}, {URI: "file:///nowhere/b.go", Range: r}}
		}
	}
	ctx := context.Background()
	for _, hook := range []godemirror.GoTo{h.e.TypeDefinition, h.e.Implementation} {
		if loc, ok := hook(ctx, h.e.State(), 4); !ok || loc.URI != "" || loc.Start.Column != 4 {
			t.Fatalf("went to %+v %v", loc, ok)
		}
	}
	refs := h.e.References(ctx, h.e.State(), 4)
	if len(refs) != 2 || refs[0].URI != "" || refs[1].URI != "file:///nowhere/b.go" {
		t.Fatalf("references %+v", refs)
	}
	p := unmarshal[map[string]json.RawMessage](t, s.wait("textDocument/references"))
	if string(p["context"]) != `{"includeDeclaration":true}` {
		t.Fatalf("references context %s", p["context"])
	}
}

func TestSignatureHelpReadsLabelsAndOffsets(t *testing.T) {
	h, s, _ := attached(t, "f(", map[string]any{
		"signatureHelpProvider": map[string]any{"triggerCharacters": []string{"("}, "retriggerCharacters": []string{","}},
	})
	if got := h.e.SignatureTriggers; len(got) != 2 {
		t.Fatalf("triggers %v", got)
	}
	answers := []string{
		`{"signatures":[{"label":"f(a int, a2 int)","parameters":[{"label":"a int"},{"label":"a2 int","documentation":"the second"}]}],"activeParameter":1}`,
		`{"signatures":[{"label":"f(é int, b int)","parameters":[{"label":[2,7]},{"label":[9,14]}],"activeParameter":0}]}`,
		`null`,
	}
	for i, a := range answers {
		s.handlers["textDocument/signatureHelp"] = func(json.RawMessage) any { return json.RawMessage(a) }
		sig, ok := h.e.SignatureHelp(context.Background(), h.e.State(), 2, "(")
		switch i {
		case 0:
			if !ok || sig.Active != 1 || sig.Params[1] != (godemirror.SignatureParam{From: 9, To: 15}) || sig.Doc != "the second" {
				t.Fatalf("labels read as %+v", sig)
			}
		case 1:
			if !ok || sig.Params[0] != (godemirror.SignatureParam{From: 2, To: 8}) || sig.Params[1] != (godemirror.SignatureParam{From: 10, To: 15}) {
				t.Fatalf("UTF-16 offsets read as %+v, want bytes", sig.Params)
			}
		case 2:
			if ok {
				t.Fatal("null read as a signature")
			}
		}
	}
	p := unmarshal[map[string]json.RawMessage](t, s.wait("textDocument/signatureHelp"))
	if string(p["context"]) != `{"isRetrigger":false,"triggerCharacter":"(","triggerKind":2}` {
		t.Fatalf("context %s", p["context"])
	}
}

func TestRenameReadsBothShapesOfAWorkspaceEdit(t *testing.T) {
	h, s, _ := attached(t, "var é = 1", map[string]any{"renameProvider": map[string]any{"prepareProvider": false}})
	edit := `{"range":{"start":{"line":0,"character":4},"end":{"line":0,"character":5}},"newText":"x"}`
	for _, a := range []string{
		`{"changes":{"file:///a.go":[` + edit + `],"file:///nowhere/b.go":[` + edit + `]}}`,
		`{"documentChanges":[{"textDocument":{"uri":"file:///a.go","version":1},"edits":[` + edit + `]},{"kind":"create","uri":"file:///c.go"},{"textDocument":{"uri":"file:///nowhere/b.go","version":null},"edits":[` + edit + `]}]}`,
	} {
		s.handlers["textDocument/rename"] = func(json.RawMessage) any { return json.RawMessage(a) }
		got, err := h.e.Rename(context.Background(), h.e.State(), 4, "x")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].URI != "" || got[0].Edits[0].End != (godemirror.LineColumn{Line: 0, Column: 6}) || got[1].URI != "file:///nowhere/b.go" {
			t.Fatalf("%s read as %+v", a, got)
		}
	}
	p := unmarshal[map[string]any](t, s.wait("textDocument/rename"))
	if p["newName"] != "x" {
		t.Fatalf("rename params %v", p)
	}
}

func TestCodeActionsResolveApplyAndRunTheirCommand(t *testing.T) {
	h, s, _ := attached(t, "x := y", map[string]any{"codeActionProvider": map[string]any{"resolveProvider": true}})
	s.handlers["textDocument/codeAction"] = func(json.RawMessage) any {
		return json.RawMessage(`[
			{"title":"Run it","command":"gopls.run","arguments":[1]},
			{"title":"Use _","kind":"quickfix","data":{"n":1}}]`)
	}
	s.handlers["codeAction/resolve"] = func(p json.RawMessage) any {
		a := unmarshal[map[string]any](t, p)
		a["edit"] = map[string]any{"changes": map[string]any{"file:///a.go": []map[string]any{{
			"range": Range{Position{0, 5}, Position{0, 6}}, "newText": "_",
		}}}}
		a["command"] = map[string]any{"title": "after", "command": "gopls.after"}
		return a
	}
	ctx := context.Background()
	st := h.e.State()
	actions := h.e.CodeActions(ctx, st, 0, 6, []godemirror.Diagnostic{{From: 5, To: 6, Message: "undefined: y"}})
	if len(actions) != 2 || actions[0].Title != "Run it" || actions[1].Kind != "quickfix" {
		t.Fatalf("actions %+v", actions)
	}
	p := unmarshal[struct {
		Context struct {
			Diagnostics []Diagnostic `json:"diagnostics"`
		} `json:"context"`
	}](t, s.wait("textDocument/codeAction"))
	if d := p.Context.Diagnostics; len(d) != 1 || d[0].Range.Start.Character != 5 {
		t.Fatalf("context diagnostics %+v", d)
	}
	edits, err := h.e.ApplyCodeAction(ctx, st, actions[1])
	if err != nil || len(edits) != 1 || edits[0].Edits[0].Text != "_" {
		t.Fatalf("edits %+v, %v", edits, err)
	}
	cmd := unmarshal[map[string]any](t, s.wait("workspace/executeCommand"))
	if cmd["command"] != "gopls.after" {
		t.Fatalf("ran %v", cmd)
	}
	if _, err := h.e.ApplyCodeAction(ctx, st, actions[0]); err != nil {
		t.Fatal(err)
	}
}

func TestTheServersApplyEditReachesTheEditor(t *testing.T) {
	h, s, _ := attached(t, "x := y", map[string]any{})
	var elsewhere []godemirror.DocumentEdit
	h.e.OnWorkspaceEdit = func(d []godemirror.DocumentEdit) { elsewhere = d }
	edit := func(uri string) map[string]any {
		return map[string]any{"textDocument": map[string]any{"uri": uri, "version": nil}, "edits": []map[string]any{{
			"range": Range{Position{0, 5}, Position{0, 6}}, "newText": "z",
		}}}
	}
	raw := s.request("workspace/applyEdit", map[string]any{"edit": map[string]any{
		"documentChanges": []any{edit("file:///a.go"), edit("file:///nowhere/b.go")},
	}})
	if string(raw) != `{"applied":true}` {
		t.Fatalf("applyEdit answered %s", raw)
	}
	h.until("applied", func() bool { return h.e.State().Doc() == "x := z" && len(elsewhere) == 1 })
}
