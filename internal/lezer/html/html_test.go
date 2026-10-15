package html

import (
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// TestMatchesLezer parses and highlights every fixture and requires the tree
// and the spans the real @lezer/html, nesting @lezer/javascript and
// @lezer/css as nesting.mjs sets it up, and @lezer/highlight gave it, as
// internal/lezer/gen/golden.mjs wrote them down.
func TestMatchesLezer(t *testing.T) {
	lezertest.MatchGoldens(t, Parser(), "testdata")
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
	lezertest.MatchCorpus(t, Parser(), "html")
}
