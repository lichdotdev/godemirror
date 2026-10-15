package java

import (
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
	"github.com/lichdotdev/godemirror/tags"
)

// TestMatchesLezer parses and highlights every fixture and requires the tree
// and the spans the real @lezer/java and @lezer/highlight gave it, as
// internal/lezer/gen/golden.mjs wrote them down.
func TestMatchesLezer(t *testing.T) {
	lezertest.MatchGoldens(t, Parser(), "testdata")
}

// TestExtraStylesMatchLezer does the same with the rules teststyles.mjs
// adds, which use what @lezer/java's own do not: inherited and opaque
// rules, several tags on a node, wildcards and deeper paths.
func TestExtraStylesMatchLezer(t *testing.T) {
	styled := spec
	styled.Styles = testStyles
	p, err := lezer.New(styled, lezer.Externals{})
	if err != nil {
		t.Fatal(err)
	}
	lezertest.MatchGoldens(t, p, "testdata/styled")
}

// TestStyleTagsMatchLezer adds teststyles.mjs's rules to the parser at run
// time, through lezer.StyleTags and WithStyleTags, as parser.configure({props:
// [styleTags(...)]}) adds them in node, and requires the same goldens.
func TestStyleTagsMatchLezer(t *testing.T) {
	byName, err := lezer.StyleTags([]lezer.StyleSpec{
		{Selector: "Block/...", Tags: []*tags.Tag{tags.Emphasis}},
		{Selector: "ClassBody/...", Tags: []*tags.Tag{tags.Strong}},
		{Selector: "LambdaExpression!", Tags: []*tags.Tag{tags.Special(tags.VariableName)}},
		{Selector: "Annotation!", Tags: []*tags.Tag{tags.Meta}},
		{Selector: "StringLiteral", Tags: []*tags.Tag{tags.String, tags.Special(tags.String)}},
		{Selector: "ClassBody/MethodDeclaration/Definition", Tags: []*tags.Tag{tags.Function(tags.Definition(tags.VariableName))}},
		{Selector: "*/ArgumentList/Identifier", Tags: []*tags.Tag{tags.Local(tags.VariableName), tags.Constant(tags.Name)}},
		{Selector: "SwitchBlock/...", Tags: []*tags.Tag{tags.ProcessingInstruction}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lezertest.MatchGoldens(t, Parser().WithStyleTags(byName), "testdata/styled")
}

// TestIncrementalMatchesLezer runs the edit sessions under
// testdata/incremental, parsing each edit incrementally, and requires the
// trees, their layout and the spans the real Lezer gave, as
// internal/lezer/gen/incremental.mjs wrote them down.
func TestIncrementalMatchesLezer(t *testing.T) {
	lezertest.MatchIncremental(t, Parser(), "testdata/incremental")
}

// TestIncrementalCorpus does the same with the randomized corpus
// lezertest.CorpusEnv names, when it is set.
func TestIncrementalCorpus(t *testing.T) {
	lezertest.MatchCorpus(t, Parser(), "java")
}
