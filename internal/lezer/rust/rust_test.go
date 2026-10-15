package rust

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// TestMatchesLezer parses and highlights every fixture and requires the tree
// and the spans the real @lezer/rust and @lezer/highlight gave it, as
// internal/lezer/gen/golden.mjs wrote them down.
func TestMatchesLezer(t *testing.T) {
	lezertest.MatchGoldens(t, Parser(), "testdata")
}

func BenchmarkParse(b *testing.B) {
	var text strings.Builder
	for _, name := range []string{"Items.rs", "Expressions.rs", "Literals.rs"} {
		src, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			b.Fatal(err)
		}
		for range 50 {
			text.Write(src)
		}
	}
	src := text.String()
	b.SetBytes(int64(len(src)))
	for b.Loop() {
		Parser().Parse(src)
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
	lezertest.MatchCorpus(t, Parser(), "rust")
}
