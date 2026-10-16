// Package lezertest checks a port against what the real Lezer gives: the
// goldens internal/lezer/gen/golden.mjs writes, one per fixture.
package lezertest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// MatchGoldens parses and highlights every fixture in dir that has a
// .golden next to it, each in its own subtest, and fails at the first line
// that differs from it. A dir with no goldens fails: it is a mistyped path,
// not a pass.
func MatchGoldens(t *testing.T, p *lezer.Parser, dir string) {
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
			wantLines := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
			CompareLines(t, Describe(p.Parse(string(text)), utf16.Encode([]rune(string(text)))), wantLines)
		})
	}
}

// Describe is what golden.mjs writes for tree, parsed from text: the tree,
// then its spans.
func Describe(tree *lezer.Tree, text []uint16) []string {
	lines := []string{"tree " + tree.String()}
	lezer.Highlight(tree, text, lezer.NewTagSets(), func(from, to int, set *lezer.TagSet) {
		names := make([]string, len(set.Tags))
		for i, tag := range set.Tags {
			names[i] = tag.String()
		}
		lines = append(lines, fmt.Sprintf("%d %d %s", from, to, strings.Join(names, " ")))
	})
	return lines
}

// CompareLines fails at the first line where got and want differ, then on
// a difference in their length.
func CompareLines(t *testing.T, got, want []string) {
	t.Helper()
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			t.Fatalf("line %d:\n got %s\nwant %s", i+1, clip(got[i]), clip(want[i]))
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d", len(got), len(want))
	}
}

// clip shortens a tree line around nothing in particular, so a failure
// stays readable; the first difference is what the message is for.
func clip(s string) string {
	const maxShown = 4000
	if len(s) > maxShown {
		return s[:maxShown] + "…"
	}
	return s
}
