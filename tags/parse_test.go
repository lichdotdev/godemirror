package tags

import "testing"

// Every standard tag, bare and under every pair of modifiers written in
// either order, reads back as itself from the name String prints.
func TestParseRoundTripsEveryStandardTag(t *testing.T) {
	if len(standard) == 0 {
		t.Fatal("no standard tags registered")
	}
	for name, tag := range standard {
		got, err := Parse(tag.String())
		if err != nil || got != tag {
			t.Fatalf("Parse(%q) = %v, %v; want the tag itself", name, got, err)
		}
		for _, a := range modifierNames {
			for _, b := range modifierNames {
				want := modifiersByName[a](modifiersByName[b](tag))
				for _, expr := range []string{want.String(), a + "(" + b + "(" + name + "))", b + "(" + a + "(" + name + "))"} {
					if got, err := Parse(expr); err != nil || got != want {
						t.Fatalf("Parse(%q) = %v, %v; want %q", expr, got, err, want)
					}
				}
			}
		}
	}
}
