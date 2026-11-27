package main

import (
	"cmp"
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/lichdotdev/godemirror"
)

// goKeywords are offered with the file's words.
var goKeywords = strings.Fields(`break case chan const continue default defer else fallthrough for func go goto
	if import interface map package range return select struct switch type var`)

var wordPattern = regexp.MustCompile(`[\pL_][\pL\pN_]*`)

// completeWords is the demo's Complete: a browser runs no language server,
// so it offers Go's keywords and the words of the file, as CodeMirror's
// completeAnyWord does, those starting with what was typed first.
func completeWords(_ context.Context, req godemirror.CompletionRequest) godemirror.CompletionResult {
	doc := req.State.Doc()
	typed := doc[req.From:req.Pos]
	seen := map[string]bool{typed: true}
	var items []godemirror.CompletionItem
	add := func(w string, kind godemirror.CompletionKind) {
		if len(w) > 1 && !seen[w] {
			seen[w] = true
			items = append(items, godemirror.CompletionItem{Label: w, Kind: kind, From: req.From, To: req.Pos})
		}
	}
	for _, k := range goKeywords {
		add(k, godemirror.CompletionKeyword)
	}
	for _, w := range wordPattern.FindAllString(doc, -1) {
		add(w, godemirror.CompletionText)
	}
	prefix := strings.ToLower(typed)
	slices.SortFunc(items, func(a, b godemirror.CompletionItem) int {
		pa, pb := strings.HasPrefix(strings.ToLower(a.Label), prefix), strings.HasPrefix(strings.ToLower(b.Label), prefix)
		if pa != pb {
			if pa {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Label, b.Label)
	})
	return godemirror.CompletionResult{Items: items}
}
