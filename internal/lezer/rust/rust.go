// Package rust is @lezer/rust's Rust grammar for internal/lezer: its parse
// tables and styleTags rules, generated from the npm package, and the ports
// of its external tokenizers (tokens.go). go generate rewrites the tables
// and the golden files the tests compare against, with LEZER_NODE_MODULES
// naming a node_modules directory set up as internal/lezer/gen/gen.mjs
// says, with @lezer/rust@1.0.3 in place of @lezer/java.
package rust

//go:generate sh -c "node ../gen/gen.mjs \"$LEZER_NODE_MODULES\" @lezer/rust rust | gofmt > spec_generated.go"
//go:generate node ../gen/golden.mjs $LEZER_NODE_MODULES @lezer/rust testdata

import (
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// Parser is the Rust parser, built the first time it is asked for.
var Parser = sync.OnceValue(func() *lezer.Parser {
	p, err := lezer.New(spec, lezer.Externals{
		Tokenizers: map[string]lezer.Tokenizer{
			"closureParam":  closureParam,
			"tpDelim":       tpDelim,
			"literalTokens": literalTokens,
			"logicAnd":      logicAnd,
		},
	})
	if err != nil {
		panic("rust: generated spec does not load: " + err.Error())
	}
	return p
})
