package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestConnectOffersUTF8AndTakesWhatTheServerPicks(t *testing.T) {
	s := newFakeServer(t, map[string]any{"positionEncoding": "utf-8"})
	c := s.connect(Config{RootDir: "/work/proj", InitializationOptions: json.RawMessage(`{"a":1}`)})
	if c.Encoding() != UTF8 {
		t.Fatalf("encoding %q, want utf-8", c.Encoding())
	}
	p := unmarshal[struct {
		RootURI      string          `json:"rootUri"`
		InitOptions  json.RawMessage `json:"initializationOptions"`
		Capabilities struct {
			General struct {
				PositionEncodings []string `json:"positionEncodings"`
			} `json:"general"`
		} `json:"capabilities"`
	}](t, s.wait("initialize"))
	if p.RootURI != "file:///work/proj" || string(p.InitOptions) != `{"a":1}` {
		t.Fatalf("initialize sent root %q, options %s", p.RootURI, p.InitOptions)
	}
	if got := p.Capabilities.General.PositionEncodings; len(got) == 0 || got[0] != "utf-8" {
		t.Fatalf("offered encodings %v, want utf-8 first", got)
	}
	s.wait("initialized")
}

func TestAServerNamingNoEncodingSpeaksUTF16(t *testing.T) {
	c := newFakeServer(t, map[string]any{}).connect(Config{})
	if c.Encoding() != UTF16 {
		t.Fatalf("encoding %q, want utf-16", c.Encoding())
	}
}

func TestSyncReadsANumberOrAnObject(t *testing.T) {
	for _, tc := range []struct {
		sync any
		kind SyncKind
		save bool
	}{
		{2, SyncIncremental, false},
		{map[string]any{"change": 1, "save": map[string]any{"includeText": true}}, SyncFull, true},
		{map[string]any{"openClose": true, "change": 2, "save": true}, SyncIncremental, false},
	} {
		c := newFakeServer(t, map[string]any{"textDocumentSync": tc.sync}).connect(Config{})
		if kind, save := c.Sync(); kind != tc.kind || save != tc.save {
			t.Errorf("sync %v: got %v %v, want %v %v", tc.sync, kind, save, tc.kind, tc.save)
		}
	}
}

func TestTheServersConfigurationRequestGetsItsSection(t *testing.T) {
	s := newFakeServer(t, map[string]any{})
	s.connect(Config{Settings: json.RawMessage(`{"gopls":{"staticcheck":true}}`)})
	raw := s.request("workspace/configuration", map[string]any{"items": []map[string]string{
		{"section": "gopls"}, {"section": "missing"}, {"section": "gopls.staticcheck"},
	}})
	if got := strings.Join(strings.Fields(string(raw)), ""); got != `[{"staticcheck":true},null,true]` {
		t.Fatalf("configuration answered %s", got)
	}
	changed := unmarshal[map[string]json.RawMessage](t, s.wait("workspace/didChangeConfiguration"))
	if string(changed["settings"]) != `{"gopls":{"staticcheck":true}}` {
		t.Fatalf("didChangeConfiguration sent %s", changed["settings"])
	}
}

func TestAnUnknownServerRequestIsMethodNotFound(t *testing.T) {
	s := newFakeServer(t, map[string]any{})
	s.connect(Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.conn.call(ctx, "x/unknown", nil, nil)
	if re, ok := err.(*ResponseError); !ok || re.Code != codeMethodNotFound {
		t.Fatalf("got %v, want method not found", err)
	}
}

func TestDiagnosticsReachTheirWatcherUntilStopped(t *testing.T) {
	s := newFakeServer(t, map[string]any{})
	c := s.connect(Config{})
	got := make(chan PublishDiagnostics, 2)
	stop := c.WatchDiagnostics("file:///a.go", func(p PublishDiagnostics) { got <- p })
	v := 3
	s.publish(PublishDiagnostics{URI: "file:///b.go"})
	s.publish(PublishDiagnostics{URI: "file:///a.go", Version: &v, Diagnostics: []Diagnostic{{Message: "boom", Severity: SeverityWarning}}})
	select {
	case p := <-got:
		if p.URI != "file:///a.go" || *p.Version != 3 || p.Diagnostics[0].Message != "boom" {
			t.Fatalf("got %+v", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no diagnostics")
	}
	stop()
	s.publish(PublishDiagnostics{URI: "file:///a.go"})
	s.request("workspace/workspaceFolders", nil) // a round trip: the publish before it was read
	if len(got) != 0 {
		t.Fatal("a stopped watch still got diagnostics")
	}
}

func TestADiagnosticCodeIsANumberOrAString(t *testing.T) {
	d := unmarshal[[]Diagnostic](t, json.RawMessage(`[{"code":2304,"message":"a"},{"code":"UndeclaredName","message":"b"},{"message":"c"}]`))
	if d[0].Code != "2304" || d[1].Code != "UndeclaredName" || d[2].Code != "" {
		t.Fatalf("codes %q %q %q", d[0].Code, d[1].Code, d[2].Code)
	}
}

func TestCompletionReadsAListOrAnArray(t *testing.T) {
	s := newFakeServer(t, map[string]any{"completionProvider": map[string]any{"triggerCharacters": []string{"."}}})
	c := s.connect(Config{})
	answers := []string{
		`{"isIncomplete":true,"items":[{"label":"Println","kind":3,"textEdit":{"insert":{"start":{"line":0,"character":4},"end":{"line":0,"character":5}},"replace":{"start":{"line":0,"character":4},"end":{"line":0,"character":9}},"newText":"Println"}}]}`,
		`[{"label":"Printf","documentation":{"kind":"markdown","value":"Printf formats."}}]`,
		`null`,
	}
	for i, a := range answers {
		s.handlers["textDocument/completion"] = func(json.RawMessage) any { return json.RawMessage(a) }
		list, err := c.Completion(context.Background(), "file:///a.go", Position{0, 5}, ".")
		if err != nil {
			t.Fatal(err)
		}
		switch i {
		case 0:
			e := list.Items[0].TextEdit
			if !list.IsIncomplete || e == nil || e.Range.End.Character != 5 {
				t.Fatalf("list %+v, want the insert range of an InsertReplaceEdit", list)
			}
		case 1:
			if list.Items[0].Documentation.Value != "Printf formats." {
				t.Fatalf("items %+v", list.Items)
			}
		case 2:
			if len(list.Items) != 0 {
				t.Fatalf("null gave %+v", list)
			}
		}
	}
	p := unmarshal[struct {
		Context map[string]any `json:"context"`
	}](t, s.wait("textDocument/completion"))
	if p.Context["triggerCharacter"] != "." || p.Context["triggerKind"] != float64(triggerCharacter) {
		t.Fatalf("context %v", p.Context)
	}
	if got := c.CompletionTriggers(); len(got) != 1 || got[0] != "." {
		t.Fatalf("triggers %v", got)
	}
}

func TestHoverContentsInEveryShapeReadAsMarkdown(t *testing.T) {
	for raw, want := range map[string]string{
		`{"kind":"markdown","value":"**x**"}`:     "**x**",
		`"plain"`:                                 "plain",
		`{"language":"go","value":"func F()"}`:    "```go\nfunc F()\n```",
		`["a",{"language":"go","value":"var x"}]`: "a\n\n```go\nvar x\n```",
	} {
		m := unmarshal[Markup](t, json.RawMessage(raw))
		if m.Value != want {
			t.Errorf("%s read as %q, want %q", raw, m.Value, want)
		}
	}
}

func TestDefinitionReadsLocationsAndLinks(t *testing.T) {
	s := newFakeServer(t, map[string]any{"definitionProvider": true})
	c := s.connect(Config{})
	r := `{"start":{"line":1,"character":2},"end":{"line":1,"character":5}}`
	for _, a := range []string{
		`{"uri":"file:///x.go","range":` + r + `}`,
		`[{"uri":"file:///x.go","range":` + r + `}]`,
		`[{"targetUri":"file:///x.go","targetRange":{"start":{"line":0,"character":0},"end":{"line":9,"character":0}},"targetSelectionRange":` + r + `}]`,
	} {
		s.handlers["textDocument/definition"] = func(json.RawMessage) any { return json.RawMessage(a) }
		locs, err := c.Definition(context.Background(), "file:///a.go", Position{})
		if err != nil {
			t.Fatal(err)
		}
		if len(locs) != 1 || locs[0].URI != "file:///x.go" || locs[0].Range.Start != (Position{1, 2}) {
			t.Fatalf("%s read as %+v", a, locs)
		}
	}
	if !c.HasDefinition() || c.HasHover() {
		t.Fatal("capabilities misread")
	}
}

func TestACancelledRequestTellsTheServer(t *testing.T) {
	s := newFakeServer(t, map[string]any{"hoverProvider": true})
	c := s.connect(Config{})
	block := make(chan struct{})
	defer close(block)
	s.handlers["textDocument/hover"] = func(json.RawMessage) any { <-block; return nil }
	ctx, cancel := context.WithCancel(context.Background())
	go func() { s.wait("textDocument/hover"); cancel() }()
	if _, err := c.Hover(ctx, "file:///a.go", Position{}); err != context.Canceled {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestCloseShutsTheServerDown(t *testing.T) {
	s := newFakeServer(t, map[string]any{})
	c := s.connect(Config{})
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	s.wait("shutdown")
	s.wait("exit")
	select {
	case <-c.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the connection outlived Close")
	}
	if _, err := c.Hover(context.Background(), "file:///a.go", Position{}); err != ErrClosed {
		t.Fatalf("a request after Close got %v, want ErrClosed", err)
	}
}

func TestFramesAreReadByContentLength(t *testing.T) {
	in := "Content-Length: 2\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n{}" +
		"Content-Length: 4\r\n\r\nnull"
	r := bufio.NewReader(strings.NewReader(in))
	for _, want := range []string{"{}", "null"} {
		body, err := readFrame(r)
		if err != nil || string(body) != want {
			t.Fatalf("read %q, %v; want %q", body, err, want)
		}
	}
}

func TestFileURIRoundTrips(t *testing.T) {
	uri := FileURI("/tmp/a b/ç.go")
	if uri != "file:///tmp/a%20b/%C3%A7.go" {
		t.Fatalf("uri %q", uri)
	}
	if p, ok := FilePath(uri); !ok || p != "/tmp/a b/ç.go" {
		t.Fatalf("path %q %v", p, ok)
	}
}
