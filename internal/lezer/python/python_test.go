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
