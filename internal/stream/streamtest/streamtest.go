// Package streamtest checks a mode's port against what CodeMirror gives:
// the goldens internal/stream/gen/golden.mjs writes, one per fixture.
package streamtest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/lezertest"
	"github.com/lichdotdev/godemirror/internal/stream"
)

// MatchGoldens highlights every fixture in dir that has a .golden next to
// it with a fresh mode from newMode, each in its own subtest, and fails at
// the first span that differs from it. A dir with no goldens fails: it is
// a mistyped path, not a pass.
func MatchGoldens(t *testing.T, newMode func() stream.Mode, dir string) {
	t.Helper()
	goldens, err := filepath.Glob(filepath.Join(dir, "*.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(goldens) == 0 {
		t.Fatalf("no golden files in %s", dir)
	}
	for _, golden := range goldens {
		fixture := strings.TrimSuffix(golden, ".golden")
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			text, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			lezertest.CompareLines(t, Describe(newMode(), string(text)), lines(string(want)))
		})
	}
}

// Describe is what golden.mjs writes for text: one line per span.
func Describe(mode stream.Mode, text string) []string {
	var spans []string
	units := utf16.Encode([]rune(text))
	stream.Highlight(units, mode, lezer.NewTagSets(), func(from, to int, set *lezer.TagSet) {
		names := make([]string, len(set.Tags))
		for i, tag := range set.Tags {
			names[i] = tag.String()
		}
		spans = append(spans, fmt.Sprintf("%d %d %s", from, to, strings.Join(names, " ")))
	})
	return spans
}

// lines is a golden's lines, none for an empty one.
func lines(golden string) []string {
	if golden == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(golden, "\n"), "\n")
}
