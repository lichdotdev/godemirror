// Package java is @lezer/java's Java grammar for internal/lezer: its parse
// tables and styleTags rules, generated from the npm package. go generate
// rewrites them, the test rules of teststyles.mjs and the golden files the
// tests compare against, with LEZER_NODE_MODULES naming a node_modules
// directory set up as internal/lezer/gen/gen.mjs says.
package java

//go:generate sh -c "node ../gen/gen.mjs \"$LEZER_NODE_MODULES\" @lezer/java java | gofmt > spec_generated.go"
//go:generate sh -c "node ../gen/gen.mjs \"$LEZER_NODE_MODULES\" @lezer/java java teststyles.mjs | gofmt > teststyles_test.go"
//go:generate node ../gen/golden.mjs $LEZER_NODE_MODULES @lezer/java testdata
//go:generate node ../gen/golden.mjs $LEZER_NODE_MODULES @lezer/java testdata/styled teststyles.mjs
//go:generate node ../gen/incremental.mjs $LEZER_NODE_MODULES @lezer/java testdata/incremental

import (
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// Parser is the Java parser, built the first time it is asked for.
var Parser = sync.OnceValue(func() *lezer.Parser {
	p, err := lezer.New(spec, lezer.Externals{})
	if err != nil {
		panic("java: generated spec does not load: " + err.Error())
	}
	return p
})
