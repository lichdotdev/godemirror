package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
)

// SyncKind is how a server wants a document's changes sent, as LSP's
// TextDocumentSyncKind.
type SyncKind int

// The sync kinds: none, the whole text each time, or the changed ranges.
const (
	SyncNone        SyncKind = 0
	SyncFull        SyncKind = 1
	SyncIncremental SyncKind = 2
)

// capabilities are the parts of the server's ServerCapabilities the client
// reads.
type capabilities struct {
	PositionEncoding   PositionEncoding `json:"positionEncoding"`
	TextDocumentSync   json.RawMessage  `json:"textDocumentSync"`
	CompletionProvider *struct {
		TriggerCharacters []string `json:"triggerCharacters"`
		ResolveProvider   bool     `json:"resolveProvider"`
	} `json:"completionProvider"`
	HoverProvider          json.RawMessage `json:"hoverProvider"`
	DefinitionProvider     json.RawMessage `json:"definitionProvider"`
	TypeDefinitionProvider json.RawMessage `json:"typeDefinitionProvider"`
	ImplementationProvider json.RawMessage `json:"implementationProvider"`
	ReferencesProvider     json.RawMessage `json:"referencesProvider"`
	SignatureHelpProvider  *struct {
		TriggerCharacters   []string `json:"triggerCharacters"`
		RetriggerCharacters []string `json:"retriggerCharacters"`
	} `json:"signatureHelpProvider"`
}

// enabled reads a capability that is a boolean or an options object.
func enabled(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && string(raw) != "false" && string(raw) != "null"
}

// Sync is how the server wants changes sent, and whether didSave carries
// the text.
func (c *Client) Sync() (kind SyncKind, saveText bool) {
	raw := bytes.TrimSpace(c.caps.TextDocumentSync)
	if len(raw) == 0 {
		return SyncNone, false
	}
	if raw[0] != '{' {
		_ = json.Unmarshal(raw, &kind)
		return kind, false
	}
	var o struct {
		Change SyncKind        `json:"change"`
		Save   json.RawMessage `json:"save"`
	}
	_ = json.Unmarshal(raw, &o)
	var save struct {
		IncludeText bool `json:"includeText"`
	}
	_ = json.Unmarshal(o.Save, &save)
	return o.Change, save.IncludeText
}

// The CompletionTriggerKinds the client sends: the user asked, or typed a
// trigger character.
const (
	invoked          = 1
	triggerCharacter = 2
)

// CompletionTriggers are the characters after which the server offers
// completions unasked.
func (c *Client) CompletionTriggers() []string {
	if c.caps.CompletionProvider == nil {
		return nil
	}
	return c.caps.CompletionProvider.TriggerCharacters
}

// HasCompletion reports whether the server answers textDocument/completion.
func (c *Client) HasCompletion() bool { return c.caps.CompletionProvider != nil }

// HasHover reports whether the server answers textDocument/hover.
func (c *Client) HasHover() bool { return enabled(c.caps.HoverProvider) }

// HasDefinition reports whether the server answers textDocument/definition.
func (c *Client) HasDefinition() bool { return enabled(c.caps.DefinitionProvider) }

// HasTypeDefinition reports whether the server answers
// textDocument/typeDefinition.
func (c *Client) HasTypeDefinition() bool { return enabled(c.caps.TypeDefinitionProvider) }

// HasImplementation reports whether the server answers
// textDocument/implementation.
func (c *Client) HasImplementation() bool { return enabled(c.caps.ImplementationProvider) }

// SignatureTriggers are the characters after which the server tells a
// call's signature, those that retrigger it included, and HasSignatureHelp
// whether it tells any.
func (c *Client) SignatureTriggers() []string {
	if p := c.caps.SignatureHelpProvider; p != nil {
		return append(slices.Clone(p.TriggerCharacters), p.RetriggerCharacters...)
	}
	return nil
}

// HasSignatureHelp reports whether the server answers
// textDocument/signatureHelp.
func (c *Client) HasSignatureHelp() bool { return c.caps.SignatureHelpProvider != nil }

// SignatureHelp asks for the signature of the call around pos; trigger is
// the character typed that asked, empty when the client asks again. nil
// when pos is in no call.
func (c *Client) SignatureHelp(ctx context.Context, uri string, pos Position, trigger string) (*SignatureHelp, error) {
	how := map[string]any{"triggerKind": invoked, "isRetrigger": trigger == ""}
	if trigger != "" {
		how["triggerKind"], how["triggerCharacter"] = triggerCharacter, trigger
	}
	var h *SignatureHelp
	err := c.conn.call(ctx, "textDocument/signatureHelp", map[string]any{
		"textDocument": textDocument{uri}, "position": pos, "context": how,
	}, &h)
	return h, err
}

// HasReferences reports whether the server answers textDocument/references.
func (c *Client) HasReferences() bool { return enabled(c.caps.ReferencesProvider) }

type textDocument struct {
	URI string `json:"uri"`
}

type versionedDocument struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type positionParams struct {
	TextDocument textDocument `json:"textDocument"`
	Position     Position     `json:"position"`
}

// DidOpen tells the server a document is open, with its text.
func (c *Client) DidOpen(uri, languageID string, version int, text string) error {
	return c.conn.send(nil, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": languageID, "version": version, "text": text,
	}})
}

// DidChange tells the server the document is now at version, after
// changes, applied in order.
func (c *Client) DidChange(uri string, version int, changes []ContentChange) error {
	return c.conn.send(nil, "textDocument/didChange", map[string]any{
		"textDocument": versionedDocument{uri, version}, "contentChanges": changes,
	})
}

// DidSave tells the server the document was saved; text, when the server
// asked for it (Sync), is what was saved.
func (c *Client) DidSave(uri string, text *string) error {
	p := map[string]any{"textDocument": textDocument{uri}}
	if text != nil {
		p["text"] = *text
	}
	return c.conn.send(nil, "textDocument/didSave", p)
}

// DidClose tells the server the document is closed.
func (c *Client) DidClose(uri string) error {
	return c.conn.send(nil, "textDocument/didClose", map[string]any{"textDocument": textDocument{uri}})
}

// Completion asks for the completions at pos. trigger is the character
// typed that asked for them, empty when the user asked.
func (c *Client) Completion(ctx context.Context, uri string, pos Position, trigger string) (CompletionList, error) {
	how := map[string]any{"triggerKind": invoked}
	if trigger != "" {
		how["triggerKind"], how["triggerCharacter"] = triggerCharacter, trigger
	}
	var list CompletionList
	err := c.conn.call(ctx, "textDocument/completion", map[string]any{
		"textDocument": textDocument{uri}, "position": pos, "context": how,
	}, &list)
	return list, err
}

// ResolveCompletion asks the server to fill in item, its documentation
// most of all; a server that does not resolve gives item back.
func (c *Client) ResolveCompletion(ctx context.Context, item CompletionItem) (CompletionItem, error) {
	if c.caps.CompletionProvider == nil || !c.caps.CompletionProvider.ResolveProvider {
		return item, nil
	}
	resolved := item
	err := c.conn.call(ctx, "completionItem/resolve", item, &resolved)
	return resolved, err
}

// Hover asks what to show for pos; nil when the server has nothing.
func (c *Client) Hover(ctx context.Context, uri string, pos Position) (*Hover, error) {
	var h *Hover
	err := c.conn.call(ctx, "textDocument/hover", positionParams{textDocument{uri}, pos}, &h)
	return h, err
}

// Definition asks where the symbol at pos is defined.
func (c *Client) Definition(ctx context.Context, uri string, pos Position) ([]Location, error) {
	return c.locations(ctx, "textDocument/definition", positionParams{textDocument{uri}, pos})
}

// TypeDefinition asks where the type of the symbol at pos is defined.
func (c *Client) TypeDefinition(ctx context.Context, uri string, pos Position) ([]Location, error) {
	return c.locations(ctx, "textDocument/typeDefinition", positionParams{textDocument{uri}, pos})
}

// Implementation asks where the symbol at pos, an interface or a method of
// one say, is implemented.
func (c *Client) Implementation(ctx context.Context, uri string, pos Position) ([]Location, error) {
	return c.locations(ctx, "textDocument/implementation", positionParams{textDocument{uri}, pos})
}

// References asks for the references to the symbol at pos, its
// declaration among them when withDeclaration is set.
func (c *Client) References(ctx context.Context, uri string, pos Position, withDeclaration bool) ([]Location, error) {
	return c.locations(ctx, "textDocument/references", map[string]any{
		"textDocument": textDocument{uri}, "position": pos,
		"context": map[string]bool{"includeDeclaration": withDeclaration},
	})
}

func (c *Client) locations(ctx context.Context, method string, params any) ([]Location, error) {
	var raw json.RawMessage
	if err := c.conn.call(ctx, method, params, &raw); err != nil {
		return nil, err
	}
	return locations(raw)
}

// section is the part of settings a dotted section names, null when there
// is none, and the whole of settings for an empty section.
func section(settings json.RawMessage, name string) json.RawMessage {
	null := json.RawMessage("null")
	if settings == nil {
		return null
	}
	cur := settings
	for part := range strings.SplitSeq(name, ".") {
		if part == "" {
			continue
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(cur, &m) != nil {
			return null
		}
		next, ok := m[part]
		if !ok {
			return null
		}
		cur = next
	}
	return cur
}
