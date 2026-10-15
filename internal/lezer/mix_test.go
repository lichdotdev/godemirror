package lezer_test

// The goldens under testdata/mix are what the real @lezer/java, nesting
// itself through @lezer/common's parseMixed as testdata/mixnest.mjs does,
// and @lezer/highlight give the fixtures; javaNest below is the same nest.

//go:generate node gen/golden.mjs --wrap=testdata/mixnest.mjs $LEZER_NODE_MODULES @lezer/java testdata/mix
//go:generate node gen/incremental.mjs --wrap=testdata/mixnest.mjs $LEZER_NODE_MODULES @lezer/java testdata/mix/incremental

import (
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/java"
	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// javaNest parses Java inside Java: a string starting "! and a line
// comment holding a ! replace their node, a block comment's inside and
// the lines after "//>" in a block are overlays.
func javaNest(n *lezer.Node, in lezer.Input) *lezer.NestedParse {
	from, to := n.From(), n.To()
	switch n.Type().Name {
	case "StringLiteral":
		if in.Read(from, from+2) == `"!` {
			return &lezer.NestedParse{Parser: java.Parser()}
		}
	case "LineComment":
		if strings.Contains(in.Read(from, to), "!") {
			return &lezer.NestedParse{Parser: java.Parser()}
		}
	case "BlockComment":
		if to-2 > from+2 {
			return &lezer.NestedParse{Parser: java.Parser(), Overlay: []lezer.Range{{From: from + 2, To: to - 2}}}
		}
	case "Block":
		return blockOverlay(n, in)
	}
	return nil
}

func blockOverlay(n *lezer.Node, in lezer.Input) *lezer.NestedParse {
	const marker = "//>"
	var overlay []lezer.Range
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Type().Name == "LineComment" && in.Read(c.From(), c.From()+len(marker)) == marker {
			overlay = append(overlay, lezer.Range{From: c.From() + len(marker), To: c.To()})
		}
	}
	if overlay == nil {
		return nil
	}
	return &lezer.NestedParse{Parser: java.Parser(), Overlay: overlay}
}

// TestMixedMatchesLezer covers replacing mounts, overlays of one and of
// several ranges, nodes an overlay covers not nesting again, and mounts on
// nodes the parse left inside buffers.
func TestMixedMatchesLezer(t *testing.T) {
	lezertest.MatchGoldens(t, java.Parser().WithNest(javaNest), "testdata/mix")
}

// TestMixedParseOfRanges parses only some ranges of the input: the nested
// parses skip the gaps too. The want is what the real stack gives for
// parser.parse(text, undefined, ranges).
func TestMixedParseOfRanges(t *testing.T) {
	text := `class A { /* int a; @@@ int b; */ String s = "!int@@@ c;"; }`
	ranges := []lezer.Range{{From: 0, To: 20}, {From: 23, To: 50}, {From: 53, To: 60}}
	tree := java.Parser().WithNest(javaNest).ParseUnits(utf16.Encode([]rune(text)), ranges, nil)
	want := []string{
		`tree Program(ClassDeclaration(class,Definition,ClassBody("{",BlockComment,FieldDeclaration(TypeName,VariableDeclarator(Definition,AssignOp,Program(ExpressionStatement(StringLiteral,⚠))),";"),"}")))`,
		"0 5 definitionKeyword", "6 7 definition(variableName)", "8 9 brace", "10 12 blockComment",
		"13 16 standard(typeName)", "17 18 definition(variableName)", "18 19 separator",
		"24 27 standard(typeName)", "28 29 definition(variableName)", "29 30 separator", "31 33 blockComment",
		"34 40 typeName", "41 42 definition(variableName)", "43 44 definitionOperator", "45 57 string",
		"57 58 separator", "59 60 brace",
	}
	lezertest.CompareLines(t, lezertest.Describe(tree), want)
}

// TestMixedIncrementalMatchesLezer runs the edit sessions under
// testdata/mix/incremental, where nested parses, overlays included, reuse
// the trees mounted in the fragments.
func TestMixedIncrementalMatchesLezer(t *testing.T) {
	lezertest.MatchIncremental(t, java.Parser().WithNest(javaNest), "testdata/mix/incremental")
}

// TestMixedIncrementalCorpus does the same with the randomized corpus
// lezertest.CorpusEnv names, when it is set.
func TestMixedIncrementalCorpus(t *testing.T) {
	lezertest.MatchCorpus(t, java.Parser().WithNest(javaNest), "mix")
}
