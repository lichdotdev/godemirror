package css

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
)

// TestMatchesLezer parses and highlights every fixture and requires the tree
// and the spans the real @lezer/css and @lezer/highlight gave it, as
// internal/lezer/gen/golden.mjs wrote them down.
func TestMatchesLezer(t *testing.T) {
	lezertest.MatchGoldens(t, Parser(), "testdata")
}

func BenchmarkParse(b *testing.B) {
	var text strings.Builder
	for _, name := range []string{"Selectors.css", "Values.css", "AtRules.css"} {
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
