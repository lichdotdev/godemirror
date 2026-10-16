// Package javascript is @lezer/javascript's JavaScript grammar for
// internal/lezer: its parse tables and styleTags rules, generated from the
// npm package, and ports of its external tokenizers and context tracker.
// go generate rewrites the tables and the golden files the tests compare
// against, with LEZER_NODE_MODULES naming a node_modules directory set up
// as internal/lezer/gen/gen.mjs says (with @lezer/javascript in place of
// @lezer/java).
//
// The grammar has two dialects, jsx and ts. Each parser here enables them
// the way @codemirror/lang-javascript's language of the same name does.
package javascript

//go:generate sh -c "node ../gen/gen.mjs \"$LEZER_NODE_MODULES\" @lezer/javascript javascript | gofmt > spec_generated.go"
//go:generate node ../gen/golden.mjs $LEZER_NODE_MODULES @lezer/javascript testdata/js
//go:generate node ../gen/golden.mjs --dialect=jsx $LEZER_NODE_MODULES @lezer/javascript testdata/jsx
//go:generate node ../gen/golden.mjs --dialect=ts $LEZER_NODE_MODULES @lezer/javascript testdata/ts
//go:generate node ../gen/golden.mjs "--dialect=jsx ts" $LEZER_NODE_MODULES @lezer/javascript testdata/tsx
//go:generate node ../gen/golden.mjs $LEZER_NODE_MODULES @lezer/javascript testdata/richer/js richer.mjs
//go:generate node ../gen/golden.mjs --dialect=ts $LEZER_NODE_MODULES @lezer/javascript testdata/richer/ts richer.mjs
//go:generate node ../gen/incremental.mjs $LEZER_NODE_MODULES @lezer/javascript testdata/js/incremental
//go:generate node ../gen/incremental.mjs --dialect=jsx $LEZER_NODE_MODULES @lezer/javascript testdata/jsx/incremental
//go:generate node ../gen/incremental.mjs --dialect=ts $LEZER_NODE_MODULES @lezer/javascript testdata/ts/incremental
//go:generate node ../gen/incremental.mjs "--dialect=jsx ts" $LEZER_NODE_MODULES @lezer/javascript testdata/tsx/incremental

import (
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// Parser is the plain JavaScript parser, built the first time it is asked
// for.
var Parser = sync.OnceValue(func() *lezer.Parser { return withDialect("") })

// JSX is the parser for JavaScript with JSX.
var JSX = sync.OnceValue(func() *lezer.Parser { return withDialect("jsx") })

// TypeScript is the parser for TypeScript.
var TypeScript = sync.OnceValue(func() *lezer.Parser { return withDialect("ts") })

// TSX is the parser for TypeScript with JSX.
var TSX = sync.OnceValue(func() *lezer.Parser { return withDialect("jsx ts") })

var base = sync.OnceValue(func() *lezer.Parser {
	p, err := lezer.New(spec, externals)
	if err != nil {
		panic("javascript: generated spec does not load: " + err.Error())
	}
	return p
})

func withDialect(dialect string) *lezer.Parser {
	p, err := base().Configure(lezer.Config{Dialect: dialect})
	if err != nil {
		panic("javascript: cannot configure dialect " + dialect + ": " + err.Error())
	}
	return p
}
