// Package golang is @lezer/go's Go grammar for internal/lezer: its parse
// tables and styleTags rules, generated from the npm package, and the ports
// of its external tokenizer and context tracker (tokens.go). go generate
// rewrites the tables and the golden files the tests compare against, with
// LEZER_NODE_MODULES naming a node_modules directory set up as
// internal/lezer/gen/gen.mjs says, with @lezer/go@1.0.1 in place of
// @lezer/java.
package golang

//go:generate sh -c "node ../gen/gen.mjs \"$LEZER_NODE_MODULES\" @lezer/go golang | gofmt > spec_generated.go"
//go:generate node ../gen/golden.mjs $LEZER_NODE_MODULES @lezer/go testdata
//go:generate node ../gen/incremental.mjs $LEZER_NODE_MODULES @lezer/go testdata/incremental

import (
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// Parser is the Go parser, built the first time it is asked for.
var Parser = sync.OnceValue(func() *lezer.Parser {
	p, err := lezer.New(spec, lezer.Externals{
		Tokenizers: map[string]lezer.Tokenizer{"semicolon": semicolon},
		Context:    &trackTokens,
	})
	if err != nil {
		panic("golang: generated spec does not load: " + err.Error())
	}
	return p
})
