package main

import (
	"context"
	"slices"
	"testing"

	"github.com/lichdotdev/godemirror"
)

func TestCompleteWordsOffersTheFilesWordsPrefixFirst(t *testing.T) {
	doc := "func Handle(w Writer) { wr := w; Wr }"
	e, err := godemirror.NewEditor("monospace", 13)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetState(godemirror.EditorStateConfig{Doc: doc}); err != nil {
		t.Fatal(err)
	}
	s := e.State()
	pos := len(doc) - 2
	r := completeWords(context.Background(), godemirror.CompletionRequest{State: s, Pos: pos, From: pos - 2})
	var labels []string
	for _, it := range r.Items {
		labels = append(labels, it.Label)
	}
	if len(labels) < 2 || !slices.Contains(labels[:2], "wr") || !slices.Contains(labels[:2], "Writer") {
		t.Fatalf("offered %v, want wr and Writer first", labels)
	}
	for _, l := range labels {
		if l == "Wr" || l == "w" {
			t.Fatalf("offered %q: the word typed and one-letter words are left out", l)
		}
	}
}
