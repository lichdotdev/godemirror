package golang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// TestMatchesLezer parses and highlights every fixture and requires the tree
// and the spans the real @lezer/go and @lezer/highlight gave it, as
// internal/lezer/gen/golden.mjs wrote them down.
func TestMatchesLezer(t *testing.T) {
	lezertest.MatchGoldens(t, Parser(), "testdata")
}

// BenchmarkParse parses program.go.txt with its declarations repeated into
// one big file, its package clause and imports on top so the copies stay
// valid Go.
func BenchmarkParse(b *testing.B) {
	const copies = 100
	src, err := os.ReadFile(filepath.Join("testdata", "program.go.txt"))
	if err != nil {
		b.Fatal(err)
	}
	header, decls, ok := strings.Cut(string(src), "\nconst Pi2")
	if !ok {
		b.Fatal("program.go.txt no longer starts its declarations with Pi2")
	}
	text := header + strings.Repeat("\nconst Pi2"+decls, copies)
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		Parser().Parse(text)
	}
}
