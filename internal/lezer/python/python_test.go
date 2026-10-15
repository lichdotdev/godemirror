package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// TestMatchesLezer parses and highlights every fixture and requires the tree
// and the spans the real @lezer/python and @lezer/highlight gave it, as
// internal/lezer/gen/golden.mjs wrote them down.
func TestMatchesLezer(t *testing.T) {
	lezertest.MatchGoldens(t, Parser(), "testdata")
}

// BenchmarkParse parses program.py repeated into one big file, which
// stays valid Python: every copy starts and ends at the top level.
func BenchmarkParse(b *testing.B) {
	const copies = 100
	src, err := os.ReadFile(filepath.Join("testdata", "program.py"))
	if err != nil {
		b.Fatal(err)
	}
	text := strings.Repeat(string(src), copies)
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		Parser().Parse(text)
	}
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
	lezertest.MatchCorpus(t, Parser(), "python")
}
