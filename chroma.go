//go:build !godemirror_nochroma

package godemirror

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// highlightOther builds the document of text for a file no Lezer grammar or
// stream mode reads, with the chroma lexer its name matches; nil when none
// does.
func highlightOther(filename, text string) (*document, error) {
	lexer := lexers.Match(filename)
	if lexer == nil {
		return nil, nil
	}
	return highlightChroma(lexer, text)
}

// highlightChroma builds the document of text with the tags tagmap.go gives
// lexer's tokens.
func highlightChroma(lexer chroma.Lexer, text string) (*document, error) {
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return nil, err
	}
	d, sets := newDocument(), lezer.NewTagSets()
	for t := it(); t != chroma.EOF; t = it() {
		d.addToken(t, sets)
	}
	// Lexers that ensure a final newline hand one back the text never had.
	lines := strings.Count(text, "\n") + 1
	d.lines, d.tags = d.lines[:lines], d.tags[:lines]
	d.source, d.cellOf = d.source[:lines], d.cellOf[:lines]
	return d, nil
}
