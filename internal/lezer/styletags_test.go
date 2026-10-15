package lezer

import (
	"fmt"
	"slices"
	"testing"

	"github.com/lichdotdev/godemirror/tags"
)

func describeRules(rules []Rule) []string {
	var out []string
	for _, r := range rules {
		names := make([]string, len(r.Tags))
		for i, t := range r.Tags {
			names[i] = t.String()
		}
		out = append(out, fmt.Sprintf("%v %d %q", names, r.Mode, r.Context))
	}
	return out
}

func mustStyleTags(t *testing.T, specs ...StyleSpec) map[string][]Rule {
	t.Helper()
	byName, err := StyleTags(specs)
	if err != nil {
		t.Fatal(err)
	}
	return byName
}

// The want is the rule chain @lezer/highlight 1.2.5's styleTags gives node X
// for the same object, printed in node.
func TestStyleTagsOrdersRulesAsLezer(t *testing.T) {
	byName := mustStyleTags(t,
		StyleSpec{Selector: "X", Tags: []*tags.Tag{tags.Keyword}},
		StyleSpec{Selector: "*/X A/X", Tags: []*tags.Tag{tags.String}},
		StyleSpec{Selector: "B/C/X X/...", Tags: []*tags.Tag{tags.Number}},
		StyleSpec{Selector: `"a/b"/X!`, Tags: []*tags.Tag{tags.Comment}},
	)
	want := []string{
		`[number] 2 ["B" "C"]`,
		`[string] 2 [""]`,
		`[string] 2 ["A"]`,
		`[comment] 0 ["a/b"]`,
		`[keyword] 2 []`,
		`[number] 1 []`,
	}
	if got := describeRules(byName["X"]); !slices.Equal(got, want) {
		t.Fatalf("rules =\n%q\nwant\n%q", got, want)
	}
}

// The want is what NodeSet.extend gives a node type styled by the first
// styleTags and then extended with the second, printed in node.
func TestAddedRulesCombineAsLezer(t *testing.T) {
	base := mustStyleTags(t,
		StyleSpec{Selector: "X", Tags: []*tags.Tag{tags.Keyword}},
		StyleSpec{Selector: "A/X B/A/X", Tags: []*tags.Tag{tags.Atom}},
	)
	added := mustStyleTags(t,
		StyleSpec{Selector: "A/X", Tags: []*tags.Tag{tags.String}},
		StyleSpec{Selector: "X", Tags: []*tags.Tag{tags.Number}},
		StyleSpec{Selector: "C/X", Tags: []*tags.Tag{tags.Comment}},
	)
	want := []string{
		`[atom] 2 ["B" "A"]`,
		`[string] 2 ["A"]`,
		`[comment] 2 ["C"]`,
		`[atom] 2 ["A"]`,
		`[number] 2 []`,
	}
	if got := describeRules(combineRules(base["X"], added["X"])); !slices.Equal(got, want) {
		t.Fatalf("rules =\n%q\nwant\n%q", got, want)
	}
}

func TestStyleTagsRefusesABadPath(t *testing.T) {
	for _, path := range []string{"*", "A/", "A!B", "A/*", `"unclosed/X`} {
		if _, err := StyleTags([]StyleSpec{{Selector: path, Tags: []*tags.Tag{tags.Keyword}}}); err == nil {
			t.Errorf("StyleTags(%q) took the path", path)
		}
	}
}
