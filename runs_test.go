package godemirror

import (
	"strings"
	"testing"
)

func TestHighlightColorsGoAsAViewDoes(t *testing.T) {
	lines, err := Highlight("x.go", "func main() {\n\treturn \"hi\"\n}", nil, DarkTheme)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	colors := map[any]bool{}
	for _, runs := range lines {
		for _, r := range runs {
			colors[r.Style.Color] = true
		}
	}
	if len(colors) < 3 {
		t.Errorf("Go is drawn in %d colors, want keywords, strings and the rest apart", len(colors))
	}
	var second strings.Builder
	for _, r := range lines[1] {
		second.WriteString(r.Text)
	}
	if got := second.String(); got != "    return \"hi\"" {
		t.Errorf("second line reads %q, want its tab expanded to the View's stop", got)
	}
}

func TestHighlightLeavesALanguageNobodyClaimsPlain(t *testing.T) {
	lines, err := Highlight("x.nolanguage", "a b\nc", nil, DarkTheme)
	if err != nil {
		t.Fatal(err)
	}
	for i, runs := range lines {
		if len(runs) != 1 || runs[0].Style.Color != DarkTheme.Foreground {
			t.Errorf("line %d = %+v, want one run in the theme's foreground", i, runs)
		}
	}
}

func BenchmarkHighlightFiveHundredLinesOfGo(b *testing.B) {
	src := strings.Repeat("func f(x int) string {\n\treturn fmt.Sprintf(\"%d\", x) // a comment\n}\n", 167)
	for b.Loop() {
		if _, err := Highlight("x.go", src, nil, DarkTheme); err != nil {
			b.Fatal(err)
		}
	}
}
