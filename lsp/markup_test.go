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

func TestPlaintextIsEscapedAsMarkdown(t *testing.T) {
	if got := markdown(Markup{Kind: "plaintext", Value: "a*b `c` [d] _e_ #f"}); got != "a\\*b \\`c\\` \\[d\\] \\_e\\_ \\#f" {
		t.Fatalf("got %q", got)
	}
	if got := markdown(Markup{Kind: "markdown", Value: "**kept**"}); got != "**kept**" {
		t.Fatalf("markdown changed to %q", got)
	}
}
