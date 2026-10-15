package java

import (
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
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
