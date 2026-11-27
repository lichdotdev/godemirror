package godemirror

import (
	"reflect"
	"testing"
)

func TestMarkdownBlocks(t *testing.T) {
	src := "```go\nfunc F(a int) error\n```\n\n# Title\n\nF does **this** and\n*that*, see [docs](http://x).\n\n- one `x`\n- two\n  continued\n1. first\n\n---\nlast  \nline"
	got := parseMarkdown(src)
	want := []mdBlock{
		{kind: codeBlock, lang: "go", code: "func F(a int) error"},
		{inlines: []inline{{text: "Title", bold: true}}},
		{inlines: []inline{{text: "F does "}, {text: "this", bold: true}, {text: " and "}, {text: "that", italic: true}, {text: ", see docs."}}},
		{bullet: "• ", inlines: []inline{{text: "one "}, {text: "x", code: true}}},
		{bullet: "• ", inlines: []inline{{text: "two continued"}}},
		{bullet: "1. ", inlines: []inline{{text: "first"}}},
		{kind: ruleBlock},
		{inlines: []inline{{text: "last"}, {text: "\n"}, {text: "line"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%+v\nwant\n%+v", got, want)
	}
}

func TestMarkdownLeavesLoneMarkersAlone(t *testing.T) {
	for src, want := range map[string][]inline{
		"returns *Ticker, or nil":    {{text: "returns *Ticker, or nil"}},
		"snake_case_name stays":      {{text: "snake_case_name stays"}},
		"a ** b":                     {{text: "a ** b"}},
		`escaped \*star\* and \_x\_`: {{text: "escaped *star* and _x_"}},
		"``a `b` c``":                {{text: "a `b` c", code: true}},
		"see <https://go.dev>":       {{text: "see https://go.dev"}},
	} {
		if got := parseInlines(src); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %+v, want %+v", src, got, want)
		}
	}
}
