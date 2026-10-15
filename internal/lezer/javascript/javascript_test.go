package javascript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// TestMatchesLezer parses and highlights every fixture with the dialects of
// its directory and requires the tree and the spans the real
// @lezer/javascript and @lezer/highlight gave it, as
// internal/lezer/gen/golden.mjs wrote them down.
func TestMatchesLezer(t *testing.T) {
	for dir, parser := range map[string]func() *lezer.Parser{
		"js": Parser, "jsx": JSX, "ts": TypeScript, "tsx": TSX,
	} {
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
