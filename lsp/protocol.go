package lsp

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

// The types below are the parts of LSP 3.17's messages the client sends or
// reads, written by hand: a field the client has no use for is left out,
// and a server sending it loses nothing but that field.

// Position is a place in a document as LSP counts it: Line from 0, and
// Character from 0 in the units of the client's PositionEncoding.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is the span from Start to End, End excluded.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range in a document, named by its URI.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Severity is how bad a Diagnostic is, 1 (error) to 4 (hint), as LSP
// numbers it.
type Severity int

// The severities of a diagnostic.
const (
	SeverityError Severity = iota + 1
	SeverityWarning
	SeverityInformation
	SeverityHint
)

// Diagnostic is a problem a server reports in a document.
type Diagnostic struct {
	Range Range `json:"range"`
	// Severity is 0 when the server leaves it to the client, which takes
	// it as an error.
	Severity Severity `json:"severity,omitempty"`
	// Code is the server's name for the problem, a number or a string as
	// the server sent it; empty when it sent none.
	Code    Code   `json:"code,omitempty"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message"`
}

// Code is a diagnostic's code, which LSP lets be a number or a string,
// read as its text.
type Code string

// UnmarshalJSON takes a number or a string.
func (c *Code) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*c = Code(s)
		return nil
	}
	if string(b) == "null" {
		*c = ""
		return nil
	}
	*c = Code(bytes.TrimSpace(b))
	return nil
}

// PublishDiagnostics is the params of textDocument/publishDiagnostics: the
// whole set of diagnostics of URI, replacing the last. Version is the
// document version they were computed for, or nil when the server does not
// say.
type PublishDiagnostics struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// ContentChange is one change of textDocument/didChange: Text replaces
// Range, or the whole document when Range is nil.
type ContentChange struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}

// CompletionItemKind is what a completion item names, as LSP numbers it.
type CompletionItemKind int

// The completion item kinds of LSP 3.17.
const (
	KindText CompletionItemKind = iota + 1
	KindMethod
	KindFunction
	KindConstructor
	KindField
	KindVariable
	KindClass
	KindInterface
	KindModule
	KindProperty
	KindUnit
	KindValue
	KindEnum
	KindKeyword
	KindSnippet
	KindColor
	KindFile
	KindReference
	KindFolder
	KindEnumMember
	KindConstant
	KindStruct
	KindEvent
	KindOperator
	KindTypeParameter
)

// InsertTextFormat says whether an item's text is plain or a snippet.
type InsertTextFormat int

// The insert text formats.
const (
	PlainText InsertTextFormat = 1
	Snippet   InsertTextFormat = 2
)

// CompletionItem is one completion a server offers.
type CompletionItem struct {
	Label            string             `json:"label"`
	Kind             CompletionItemKind `json:"kind,omitempty"`
	Detail           string             `json:"detail,omitempty"`
	Documentation    Markup             `json:"documentation,omitzero"`
	SortText         string             `json:"sortText,omitempty"`
	FilterText       string             `json:"filterText,omitempty"`
	InsertText       string             `json:"insertText,omitempty"`
	InsertTextFormat InsertTextFormat   `json:"insertTextFormat,omitempty"`
	TextEdit         *TextEdit          `json:"textEdit,omitempty"`
	// Data is the server's own, sent back with completionItem/resolve.
	Data json.RawMessage `json:"data,omitempty"`
}

// TextEdit replaces Range with NewText. Read from an InsertReplaceEdit, it
// is the edit's insert range.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// UnmarshalJSON takes a TextEdit or an InsertReplaceEdit.
func (t *TextEdit) UnmarshalJSON(b []byte) error {
	var e struct {
		Range   *Range `json:"range"`
		Insert  *Range `json:"insert"`
		NewText string `json:"newText"`
	}
	if err := json.Unmarshal(b, &e); err != nil {
		return err
	}
	t.NewText = e.NewText
	switch {
	case e.Range != nil:
		t.Range = *e.Range
	case e.Insert != nil:
		t.Range = *e.Insert
	}
	return nil
}

// CompletionList is what textDocument/completion answers: items, and
// whether typing more should ask again rather than filter them.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// UnmarshalJSON takes a CompletionList, a bare array of items, or null.
func (l *CompletionList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '[' {
		*l = CompletionList{}
		return json.Unmarshal(b, &l.Items)
	}
	type list CompletionList
	return json.Unmarshal(b, (*list)(l))
}

// Markup is text a server sends for people to read: Value is markdown when
// Kind is "markdown", plain text otherwise.
type Markup struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// UnmarshalJSON takes a MarkupContent, a MarkedString (a string or a
// language and value), or an array of MarkedStrings, joined as markdown.
func (m *Markup) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case len(b) == 0 || string(b) == "null":
		*m = Markup{}
		return nil
	case b[0] == '"':
		*m = Markup{Kind: "markdown"}
		return json.Unmarshal(b, &m.Value)
	case b[0] == '[':
		var parts []Markup
		if err := json.Unmarshal(b, &parts); err != nil {
			return err
		}
		values := make([]string, len(parts))
		for i, p := range parts {
			values[i] = p.Value
		}
		*m = Markup{Kind: "markdown", Value: strings.Join(values, "\n\n")}
		return nil
	}
	var raw struct {
		Kind     string `json:"kind"`
		Value    string `json:"value"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*m = Markup{Kind: raw.Kind, Value: raw.Value}
	if raw.Kind == "" {
		*m = Markup{Kind: "markdown", Value: "```" + raw.Language + "\n" + raw.Value + "\n```"}
	}
	return nil
}

// Hover is what textDocument/hover answers. Range, when set, is the span
// the hover is about.
type Hover struct {
	Contents Markup `json:"contents"`
	Range    *Range `json:"range,omitempty"`
}

// WorkspaceEdit is edits to documents, as a rename or a code action makes
// them: by URI in Changes, or in order in DocumentChanges, whose file
// operations (create, rename, delete) the client does not make.
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes"`
	DocumentChanges []json.RawMessage     `json:"documentChanges"`
}

// documentEdits are the text edits of w, by document, in order.
func (w WorkspaceEdit) documentEdits() []documentEdit {
	var out []documentEdit
	for _, raw := range w.DocumentChanges {
		var d struct {
			TextDocument textDocument `json:"textDocument"`
			Edits        []TextEdit   `json:"edits"`
		}
		if json.Unmarshal(raw, &d) == nil && d.TextDocument.URI != "" {
			out = append(out, documentEdit{d.TextDocument.URI, d.Edits})
		}
	}
	if len(w.DocumentChanges) > 0 {
		return out
	}
	for uri, edits := range w.Changes {
		out = append(out, documentEdit{uri, edits})
	}
	slices.SortFunc(out, func(a, b documentEdit) int { return strings.Compare(a.uri, b.uri) })
	return out
}

type documentEdit struct {
	uri   string
	edits []TextEdit
}

// Command is a command a server runs with workspace/executeCommand.
type Command struct {
	Title     string            `json:"title"`
	Command   string            `json:"command"`
	Arguments []json.RawMessage `json:"arguments,omitempty"`
}

// CodeAction is an action a server offers on a range: an edit, a command
// to run after it, or both; or, with Edit and Command both nil and
// IsCommand set, a bare command, whose Title and Command are in Command.
type CodeAction struct {
	Title       string          `json:"title"`
	Kind        string          `json:"kind,omitempty"`
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit  `json:"edit,omitempty"`
	Command     *Command        `json:"command,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	IsCommand   bool            `json:"-"`
}

// UnmarshalJSON takes a CodeAction or a bare Command.
func (a *CodeAction) UnmarshalJSON(b []byte) error {
	var probe struct {
		Command json.RawMessage `json:"command"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return err
	}
	if len(probe.Command) > 0 && probe.Command[0] == '"' {
		var c Command
		err := json.Unmarshal(b, &c)
		*a = CodeAction{Title: c.Title, Command: &c, IsCommand: true}
		return err
	}
	type action CodeAction
	return json.Unmarshal(b, (*action)(a))
}

// SignatureHelp is what textDocument/signatureHelp answers: the
// signatures of the call, the active one, and its active parameter.
type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter *int                   `json:"activeParameter"`
}

// SignatureInformation is one signature: its label, its documentation, its
// parameters, and, overriding the help's, its active parameter.
type SignatureInformation struct {
	Label           string                 `json:"label"`
	Documentation   Markup                 `json:"documentation,omitzero"`
	Parameters      []ParameterInformation `json:"parameters"`
	ActiveParameter *int                   `json:"activeParameter"`
}

// ParameterInformation is a parameter of a signature. Its label is a
// substring of the signature's, or Start and End, offsets in it in the
// position encoding's units, when the server sends offsets.
type ParameterInformation struct {
	Label         string
	Start, End    int
	Offsets       bool
	Documentation Markup `json:"documentation,omitzero"`
}

// UnmarshalJSON takes a label that is a string or a pair of offsets.
func (p *ParameterInformation) UnmarshalJSON(b []byte) error {
	var raw struct {
		Label         json.RawMessage `json:"label"`
		Documentation Markup          `json:"documentation"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*p = ParameterInformation{Documentation: raw.Documentation}
	var offsets [2]int
	if json.Unmarshal(raw.Label, &offsets) == nil {
		p.Start, p.End, p.Offsets = offsets[0], offsets[1], true
		return nil
	}
	return json.Unmarshal(raw.Label, &p.Label)
}

// locations reads what textDocument/definition answers: a Location, an
// array of Locations or of LocationLinks, or null.
func locations(b json.RawMessage) ([]Location, error) {
	b = bytes.TrimSpace(b)
	switch {
	case len(b) == 0 || string(b) == "null":
		return nil, nil
	case b[0] == '{':
		var l Location
		err := json.Unmarshal(b, &l)
		return []Location{l}, err
	}
	var raw []struct {
		Location
		TargetURI            string `json:"targetUri"`
		TargetSelectionRange *Range `json:"targetSelectionRange"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := make([]Location, len(raw))
	for i, r := range raw {
		out[i] = r.Location
		if r.TargetURI != "" && r.TargetSelectionRange != nil {
			out[i] = Location{URI: r.TargetURI, Range: *r.TargetSelectionRange}
		}
	}
	return out, nil
}
