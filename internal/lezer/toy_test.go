package lezer_test

// testdata/grammar is a small grammar that uses what the runtime supports
// and @lezer/java does not: dialects, a second top rule, external
// tokenizers (contextual, fallback and extending), an external
// specializer, a context tracker, local token groups, ambiguity with
// dynamic precedence, and styleTags rules of every kind. go generate builds
// it with the real lezer-generator into a package under LEZER_NODE_MODULES,
// set up as gen/gen.mjs says with @lezer/generator@1.8.1 added, then
// writes its spec and the goldens the real @lezer/lr and @lezer/highlight
// give the fixtures under testdata/toy.

//go:generate mkdir -p $LEZER_NODE_MODULES/lezer-testgrammar
//go:generate sh -c "cp testdata/grammar/* \"$LEZER_NODE_MODULES/lezer-testgrammar\" && cd \"$LEZER_NODE_MODULES/lezer-testgrammar\" && ../.bin/lezer-generator toy.grammar -o parser.js"
//go:generate sh -c "node gen/gen.mjs \"$LEZER_NODE_MODULES\" lezer-testgrammar lezer_test | gofmt > toyspec_test.go"
//go:generate node gen/golden.mjs $LEZER_NODE_MODULES lezer-testgrammar testdata/toy/plain
//go:generate node gen/golden.mjs --dialect=strict $LEZER_NODE_MODULES lezer-testgrammar testdata/toy/strict
//go:generate node gen/golden.mjs --top=SingleExpression $LEZER_NODE_MODULES lezer-testgrammar testdata/toy/expression

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// Term ids of testdata/grammar, from the parser.terms.js lezer-generator
// writes next to its parser.
const (
	termInsertedSemi = 61
	termSpread       = 1
	termTypeName     = 2
	termShallowMark  = 3
	termDeepMark     = 4
	termKwLet        = 5
	termKwIf         = 6
	termKwElse       = 7
	termKwAs         = 8
	termBlock        = 49
	dialectStrict    = 0
)

const (
	charNewline    = '\n'
	charCloseBrace = '}'
	charOpenBrace  = '{'
	charDot        = '.'
	charAt         = '@'
	charBang       = '!'
	spreadLength   = 3
)

// toyExternals ports testdata/grammar/tokens.js.
func toyExternals() lezer.Externals {
	keywords := map[string]int{"let": termKwLet, "if": termKwIf, "else": termKwElse}
	return lezer.Externals{
		Tokenizers: map[string]lezer.Tokenizer{
			"insertSemicolon": {Token: insertSemicolon, Contextual: true, Fallback: true},
			"spread":          {Token: spread},
			"typeName":        {Token: typeName, Extend: true},
			"marker":          {Token: marker},
		},
		Specializers: map[string]lezer.Specializer{
			"keywords": func(text string, _ *lezer.Stack) int {
				if term, ok := keywords[text]; ok {
					return term
				}
				return -1
			},
			"contextualKeywords": func(text string, _ *lezer.Stack) int {
				if text == "as" {
					return termKwAs
				}
				return -1
			},
		},
		Context: &lezer.ContextTracker{
			Start: 0,
			Shift: func(context any, _ int, _ *lezer.Stack, in *lezer.InputStream) any {
				if in.Next == charOpenBrace {
					return context.(int) + 1
				}
				return context
			},
			Reduce: func(context any, term int, _ *lezer.Stack, _ *lezer.InputStream) any {
				if term == termBlock {
					return context.(int) - 1
				}
				return context
			},
			Hash: func(context any) int { return context.(int) },
		},
	}
}

func insertSemicolon(in *lezer.InputStream, s *lezer.Stack) {
	next := in.Next
	if (next == charCloseBrace || next == -1 || next == charNewline) && s.CanShift(termInsertedSemi) {
		in.AcceptToken(termInsertedSemi, 0)
	}
}

func spread(in *lezer.InputStream, _ *lezer.Stack) {
	if in.Next == charDot && in.Peek(1) == charDot && in.Peek(2) == charDot {
		in.AcceptToken(termSpread, spreadLength)
	}
}

func typeName(in *lezer.InputStream, _ *lezer.Stack) {
	if !isUpper(in.Next) {
		return
	}
	length := 1
	for isUpper(in.Peek(length)) || isLower(in.Peek(length)) {
		length++
	}
	in.AcceptToken(termTypeName, length)
}

func isUpper(ch int) bool { return ch >= 'A' && ch <= 'Z' }

func isLower(ch int) bool { return ch >= 'a' && ch <= 'z' }

func marker(in *lezer.InputStream, s *lezer.Stack) {
	if in.Next != charAt {
		return
	}
	strict := in.Peek(1) == charBang
	if strict && !s.DialectEnabled(dialectStrict) {
		return
	}
	term, length := termShallowMark, 1
	if s.Context().(int) > 1 {
		term = termDeepMark
	}
	if strict {
		length = 2
	}
	in.AcceptToken(term, length)
}

func toyParser(t *testing.T) *lezer.Parser {
	t.Helper()
	p, err := lezer.New(spec, toyExternals())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestToyMatchesLezer requires the trees and spans the real @lezer/lr and
// @lezer/highlight give the fixtures, with each configuration golden.mjs
// wrote them with, all configured from one parser.
func TestToyMatchesLezer(t *testing.T) {
	base := toyParser(t)
	configs := map[string]lezer.Config{
		"plain":      {},
		"strict":     {Dialect: "strict"},
		"expression": {Top: "SingleExpression"},
	}
	for dir, config := range configs {
		t.Run(dir, func(t *testing.T) {
			p, err := base.Configure(config)
			if err != nil {
				t.Fatal(err)
			}
			lezertest.MatchGoldens(t, p, filepath.Join("testdata", "toy", dir))
		})
	}
}

func TestNewNamesTheMissingExternal(t *testing.T) {
	full := toyExternals()
	cases := map[string]func(*lezer.Externals){
		`"spread"`:   func(e *lezer.Externals) { delete(e.Tokenizers, "spread") },
		`"keywords"`: func(e *lezer.Externals) { e.Specializers = nil },
		`"depth"`:    func(e *lezer.Externals) { e.Context = nil },
	}
	for name, drop := range cases {
		ext := full
		ext.Tokenizers = map[string]lezer.Tokenizer{}
		for k, v := range full.Tokenizers {
			ext.Tokenizers[k] = v
		}
		drop(&ext)
		_, err := lezer.New(spec, ext)
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("without %s: got error %v, want one naming it", name, err)
		}
	}
}

func TestConfigureRejectsAnUnknownTopRule(t *testing.T) {
	if _, err := toyParser(t).Configure(lezer.Config{Top: "Nope"}); err == nil {
		t.Fatal("configured an unknown top rule")
	}
}

// TestParserIsSafeToShare parses from many goroutines at once with one
// parser, for the race detector, and requires every tree to be the same.
func TestParserIsSafeToShare(t *testing.T) {
	p := toyParser(t)
	text, err := os.ReadFile(filepath.Join("testdata", "toy", "plain", "statements.toy"))
	if err != nil {
		t.Fatal(err)
	}
	want := p.Parse(string(text)).String()
	const goroutines = 8
	var wg sync.WaitGroup
	got := make([]string, goroutines)
	for i := range goroutines {
		wg.Go(func() { got[i] = p.Parse(string(text)).String() })
	}
	wg.Wait()
	for i, tree := range got {
		if tree != want {
			t.Fatalf("goroutine %d got another tree", i)
		}
	}
}
