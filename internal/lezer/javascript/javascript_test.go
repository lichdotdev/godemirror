package javascript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// dialects are the parsers of the testdata directories.
var dialects = map[string]func() *lezer.Parser{
	"js": Parser, "jsx": JSX, "ts": TypeScript, "tsx": TSX,
}

// TestMatchesLezer parses and highlights every fixture with the dialects of
// its directory and requires the tree and the spans the real
// @lezer/javascript and @lezer/highlight gave it, as
// internal/lezer/gen/golden.mjs wrote them down.
func TestMatchesLezer(t *testing.T) {
	for dir, parser := range dialects {
		t.Run(dir, func(t *testing.T) { lezertest.MatchGoldens(t, parser(), filepath.Join("testdata", dir)) })
	}
}

func BenchmarkParse(b *testing.B) {
	text, err := os.ReadFile("testdata/ts/types.ts")
	if err != nil {
		b.Fatal(err)
	}
	const copies = 200
	big := strings.Repeat(string(text), copies)
	p := TSX()
	b.SetBytes(int64(len(big)))
	for b.Loop() {
		p.Parse(big)
	}
}

// TestIncrementalMatchesLezer runs the edit sessions under each dialect's
// testdata/incremental, parsing each edit incrementally, and requires the
// trees, their layout and the spans the real Lezer gave, as
// internal/lezer/gen/incremental.mjs wrote them down.
func TestIncrementalMatchesLezer(t *testing.T) {
	for dir, parser := range dialects {
		t.Run(dir, func(t *testing.T) {
			lezertest.MatchIncremental(t, parser(), filepath.Join("testdata", dir, "incremental"))
		})
	}
}

// TestIncrementalCorpus does the same with the randomized corpus
// lezertest.CorpusEnv names, when it is set.
func TestIncrementalCorpus(t *testing.T) {
	for dir, parser := range dialects {
		t.Run(dir, func(t *testing.T) { lezertest.MatchCorpus(t, parser(), dir) })
	}
}
