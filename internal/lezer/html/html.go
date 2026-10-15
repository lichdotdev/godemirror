// Package html is @lezer/html's HTML grammar for internal/lezer: its parse
// tables and styleTags rules, generated from the npm package, the Go ports
// of its external tokenizers and context tracker, and the nesting of
// JavaScript and CSS that @codemirror/lang-html sets up by default. go
// generate rewrites the tables and the golden files the tests compare
// against, with LEZER_NODE_MODULES naming a node_modules directory set up
// as internal/lezer/gen/gen.mjs says, with @lezer/html@1.3.13,
// @lezer/javascript@1.5.6 and @lezer/css@1.3.9 in place of @lezer/java.
package html

//go:generate sh -c "node ../gen/gen.mjs \"$LEZER_NODE_MODULES\" @lezer/html html | gofmt > spec_generated.go"
//go:generate node ../gen/golden.mjs --wrap=nesting.mjs $LEZER_NODE_MODULES @lezer/html testdata

import (
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// Parser is the HTML parser, built the first time it is asked for, with
// script, style, style attributes and event handler attributes parsed as
// @codemirror/lang-html's html() parses them by default.
var Parser = sync.OnceValue(func() *lezer.Parser {
	p, err := lezer.New(spec, externals)
	if err != nil {
		panic("html: generated spec does not load: " + err.Error())
	}
	return p.WithNest(nest)
})
