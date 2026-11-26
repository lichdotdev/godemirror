package lsp

import "testing"

func TestSnippetsLoseTheirPlaceholders(t *testing.T) {
	for in, want := range map[string]string{
		"Printf(${1:format}, ${2:a})$0":         "Printf(format, a)",
		"for ${1:i} := range ${2:n} {\n\t$0\n}": "for i := range n {\n\t\n}",
		"${1|one,two|}":                         "one",
		`cost \$5 ${1:a\}b}`:                    "cost $5 a}b",
		"${1:outer ${2:inner}}":                 "outer inner",
		"${TM_FILENAME}":                        "",
		"plain":                                 "plain",
	} {
		if got := stripSnippet(in); got != want {
			t.Errorf("stripSnippet(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlainTextDropsMarkdown(t *testing.T) {
	got := plainText(Markup{Kind: "markdown", Value: "# Title\n\nUse `x` and [docs](http://a).\n\n```go\ncode()\n```\n---\nescaped \\*star\\*"})
	if want := "Title\n\nUse x and docs.\n\ncode()\n\nescaped *star*"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := plainText(Markup{Kind: "plaintext", Value: "`kept`"}); got != "`kept`" {
		t.Fatalf("plaintext changed to %q", got)
	}
}
