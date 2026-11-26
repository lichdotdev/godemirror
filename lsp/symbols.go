package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/lichdotdev/godemirror"
)

// SymbolKind is what a symbol is, as LSP numbers it.
type SymbolKind int

// The symbol kinds of LSP 3.17.
const (
	SymbolFile SymbolKind = iota + 1
	SymbolModule
	SymbolNamespace
	SymbolPackage
	SymbolClass
	SymbolMethod
	SymbolProperty
	SymbolField
	SymbolConstructor
	SymbolEnum
	SymbolInterface
	SymbolFunction
	SymbolVariable
	SymbolConstant
	SymbolString
	SymbolNumber
	SymbolBoolean
	SymbolArray
	SymbolObject
	SymbolKey
	SymbolNull
	SymbolEnumMember
	SymbolStruct
	SymbolEvent
	SymbolOperator
	SymbolTypeParameter
)

// Symbol is a symbol of a document, for an outline: Range is all of it,
// its body included, and Selection its name, both in the editor's units
// (lines from 0, columns in UTF-8 bytes); Children are the symbols inside
// it.
type Symbol struct {
	Name, Detail     string
	Kind             SymbolKind
	Range, Selection godemirror.Location
	Children         []Symbol
}

// ErrNoSymbols is what Symbols returns when the server lists no symbols.
var ErrNoSymbols = errors.New("lsp: the server does not list document symbols")

// documentSymbol is a symbol as the server sends it, as a DocumentSymbol
// or a SymbolInformation (which has a Location and no children).
type documentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Location       *Location        `json:"location"`
	Children       []documentSymbol `json:"children"`
}

// HasSymbols reports whether the server answers
// textDocument/documentSymbol.
func (c *Client) HasSymbols() bool { return enabled(c.caps.SymbolProvider) }

// Symbols asks the server for the document's symbols, as a tree when the
// server nests them, and reads them in the editor's units. It blocks until
// the server answers, on the goroutine that lays the editor out, so ctx
// should carry a deadline.
func (d *Document) Symbols(ctx context.Context) ([]Symbol, error) {
	if !d.c.HasSymbols() {
		return nil, ErrNoSymbols
	}
	s := d.e.State()
	var raw json.RawMessage
	if err := d.c.conn.call(ctx, "textDocument/documentSymbol", map[string]any{"textDocument": textDocument{d.uri}}, &raw); err != nil {
		return nil, err
	}
	var syms []documentSymbol
	if raw = bytes.TrimSpace(raw); len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &syms); err != nil {
			return nil, err
		}
	}
	return d.symbols(s, syms), nil
}

func (d *Document) symbols(s godemirror.EditorState, in []documentSymbol) []Symbol {
	files := map[string][]string{}
	var out []Symbol
	for _, x := range in {
		r, sel := x.Range, x.SelectionRange
		uri := d.uri
		if x.Location != nil {
			uri, r, sel = x.Location.URI, x.Location.Range, x.Location.Range
		}
		out = append(out, Symbol{
			Name: x.Name, Detail: x.Detail, Kind: x.Kind,
			Range:     d.location(s, Location{URI: uri, Range: r}, files),
			Selection: d.location(s, Location{URI: uri, Range: sel}, files),
			Children:  d.symbols(s, x.Children),
		})
	}
	return out
}
