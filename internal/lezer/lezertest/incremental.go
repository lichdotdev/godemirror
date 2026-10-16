package lezertest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

// CorpusEnv names the directory of a randomized edit corpus written by
// gen/incremental.mjs, too big for the repository: one subdirectory of
// sessions and goldens per grammar. MatchCorpus skips when it is unset.
const CorpusEnv = "LEZER_INCREMENTAL_CORPUS"

// Session is an edit session as gen/incremental.mjs reads it: a document and
// the steps of edits made to it, positions in UTF-16 code units into the
// document as it was before the step.
type Session struct {
	Text  string   `json:"text"`
	Steps [][]Edit `json:"steps"`
}

// Edit replaces [From, To) with Insert.
type Edit struct {
	From   int    `json:"from"`
	To     int    `json:"to"`
	Insert string `json:"insert"`
}

// MatchIncremental runs every session in dir that has a .golden next to it,
// each in its own subtest, and fails at the first parse whose tree, layout
// or spans, or the spans of a fresh parse of the same text, differ from what
// the real Lezer gave. A dir with no goldens fails.
func MatchIncremental(t *testing.T, p *lezer.Parser, dir string) {
	t.Helper()
	goldens, err := filepath.Glob(filepath.Join(dir, "*.json.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(goldens) == 0 {
		t.Fatalf("no golden files in %s", dir)
	}
	for _, golden := range goldens {
		t.Run(filepath.Base(strings.TrimSuffix(golden, ".golden")), func(t *testing.T) {
			matchSession(t, p, golden)
		})
	}
}

// MatchCorpus is MatchIncremental over the subdirectory grammar of the
// corpus CorpusEnv names, reporting how many sessions and parses it checked
// and at how many steps Lezer's incremental parse highlights otherwise than
// a fresh one.
func MatchCorpus(t *testing.T, p *lezer.Parser, grammar string) {
	t.Helper()
	root := os.Getenv(CorpusEnv)
	if root == "" {
		t.Skip(CorpusEnv + " is not set")
	}
	dir := filepath.Join(root, grammar)
	MatchIncremental(t, p, dir)
	goldens, _ := filepath.Glob(filepath.Join(dir, "*.json.golden"))
	parses, unlikeFresh := 0, 0
	for _, g := range goldens {
		data, err := os.ReadFile(g)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(string(data)) {
			parses++
			if f := strings.Fields(line); f[3] != f[4] {
				unlikeFresh++
			}
		}
	}
	t.Logf("%s: %d sessions, %d parses, %d unlike a fresh parse", grammar, len(goldens), parses, unlikeFresh)
}

func matchSession(t *testing.T, p *lezer.Parser, golden string) {
	t.Helper()
	data, err := os.ReadFile(strings.TrimSuffix(golden, ".golden"))
	if err != nil {
		t.Fatal(err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	wantLines := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
	if len(wantLines) != len(s.Steps)+1 {
		t.Fatalf("golden has %d lines for %d steps", len(wantLines), len(s.Steps))
	}
	units := utf16.Encode([]rune(s.Text))
	tree := p.ParseUnits(units, nil, nil)
	checkStep(t, 0, units, tree, tree, wantLines[0])
	for i, edits := range s.Steps {
		var changes []lezer.ChangedRange
		units, changes = Apply(units, edits)
		fragments := lezer.ApplyChanges(lezer.AddTree(tree, nil, false), changes)
		tree = p.ParseUnits(units, nil, fragments)
		checkStep(t, i+1, units, tree, p.ParseUnits(units, nil, nil), wantLines[i+1])
	}
}

// checkStep compares a parse of text, and fresh, a fresh parse of it, with
// its golden line.
func checkStep(t *testing.T, step int, text []uint16, tree, fresh *lezer.Tree, want string) {
	t.Helper()
	got := fmt.Sprintf("%d %s %s %s %s", step, hash(tree.String()), hash(tree.Layout()), spansHash(tree, text), spansHash(fresh, text))
	if got == want {
		return
	}
	parts, wantParts := strings.Fields(got), strings.Fields(want)
	for i, name := range []string{"step", "tree", "layout", "spans", "fresh parse's spans"} {
		if i < len(wantParts) && parts[i] != wantParts[i] {
			t.Fatalf("step %d: the %s differ from Lezer's", step, name)
		}
	}
	t.Fatalf("step %d: got %q, want %q", step, got, want)
}

func spansHash(tree *lezer.Tree, text []uint16) string {
	return hash(strings.Join(Describe(tree, text)[1:], "\n"))
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Apply makes the edits, in order and not overlapping, to units, and
// returns the result with the ChangedRanges they are.
func Apply(units []uint16, edits []Edit) ([]uint16, []lezer.ChangedRange) {
	var out []uint16
	var changes []lezer.ChangedRange
	at, shift := 0, 0
	for _, e := range edits {
		insert := utf16.Encode([]rune(e.Insert))
		out = append(append(out, units[at:e.From]...), insert...)
		changes = append(changes, lezer.ChangedRange{FromA: e.From, ToA: e.To, FromB: e.From + shift, ToB: e.From + shift + len(insert)})
		shift += len(insert) - (e.To - e.From)
		at = e.To
	}
	return append(out, units[at:]...), slices.Clip(changes)
}
