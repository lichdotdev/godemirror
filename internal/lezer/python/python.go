// Package python is @lezer/python's Python grammar for internal/lezer: its
// parse tables and styleTags rules, generated from the npm package, and the
// ports of its external tokenizers and indentation tracker (tokens.go). go
// generate rewrites the tables and the golden files the tests compare
// against, with LEZER_NODE_MODULES naming a node_modules directory set up as
// internal/lezer/gen/gen.mjs says, with @lezer/python@1.1.19 in place of
// @lezer/java.
package python

//go:generate sh -c "node ../gen/gen.mjs \"$LEZER_NODE_MODULES\" @lezer/python python | gofmt > spec_generated.go"
//go:generate node ../gen/golden.mjs $LEZER_NODE_MODULES @lezer/python testdata
//go:generate node ../gen/incremental.mjs $LEZER_NODE_MODULES @lezer/python testdata/incremental

import (
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// Parser is the Python parser, built the first time it is asked for.
var Parser = sync.OnceValue(func() *lezer.Parser {
	p, err := lezer.New(spec, lezer.Externals{
		Tokenizers: map[string]lezer.Tokenizer{
			"legacyPrint": legacyPrint,
			"indentation": indentation,
			"newlines":    newlines,
			"strings":     stringTokens,
		},
		Context: &trackIndent,
	})
	if err != nil {
		panic("python: generated spec does not load: " + err.Error())
	}
	return p
})
