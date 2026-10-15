// Package css is @lezer/css's CSS grammar for internal/lezer: its parse
// tables and styleTags rules, generated from the npm package, and the Go
// ports of its external tokenizers. go generate rewrites the tables and the
// golden files the tests compare against, with LEZER_NODE_MODULES naming a
// node_modules directory set up as internal/lezer/gen/gen.mjs says (with
// @lezer/css@1.3.9 in place of @lezer/java).
package css

//go:generate sh -c "node ../gen/gen.mjs \"$LEZER_NODE_MODULES\" @lezer/css css | gofmt > spec_generated.go"
//go:generate node ../gen/golden.mjs $LEZER_NODE_MODULES @lezer/css testdata
//go:generate node ../gen/incremental.mjs $LEZER_NODE_MODULES @lezer/css testdata/incremental

import (
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// Parser is the CSS parser for a style sheet, built the first time it is
// asked for. Configure it with Top "Styles" for the declarations of a
// style attribute.
var Parser = sync.OnceValue(func() *lezer.Parser {
	p, err := lezer.New(spec, externals)
	if err != nil {
		panic("css: generated spec does not load: " + err.Error())
	}
	return p
})
