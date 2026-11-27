package lsp

import (
	"context"
	"errors"
	"strings"
)

// defaultTabSize is the tab size a document indented with tabs is
// formatted with, CodeMirror's tabSize default.
const defaultTabSize = 4

// ErrNoFormatting is what Format returns when the server does not format.
var ErrNoFormatting = errors.New("lsp: the server does not format documents")

// Format asks the server to format the whole document, indented as the
// editor's IndentUnit, and makes its edits as one transaction, which one
// Undo takes back. It blocks until the server answers, on the goroutine
// that lays the editor out, so ctx should carry a deadline. Formatting on
// save is the caller's: Format, then write the document, then Save.
func (d *Document) Format(ctx context.Context) error {
	if !d.c.HasFormatting() {
		return ErrNoFormatting
	}
	s := d.e.State()
	unit := s.IndentUnit()
	opts := FormattingOptions{TabSize: len(unit), InsertSpaces: true}
	if strings.HasPrefix(unit, "\t") {
		opts = FormattingOptions{TabSize: defaultTabSize}
	}
	edits, err := d.c.Format(ctx, d.uri, opts)
	if err != nil {
		return err
	}
	d.e.ApplyEdits(d.editorEdits(s, []documentEdit{{d.uri, edits}}))
	return nil
}
