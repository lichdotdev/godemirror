package lsp

import "testing"

func TestPlaintextIsEscapedAsMarkdown(t *testing.T) {
	if got := markdown(Markup{Kind: "plaintext", Value: "a*b `c` [d] _e_ #f"}); got != "a\\*b \\`c\\` \\[d\\] \\_e\\_ \\#f" {
		t.Fatalf("got %q", got)
	}
	if got := markdown(Markup{Kind: "markdown", Value: "**kept**"}); got != "**kept**" {
		t.Fatalf("markdown changed to %q", got)
	}
}
