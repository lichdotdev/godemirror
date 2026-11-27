package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/godemirror"
)

// completeAt runs complete on doc with the cursor at the end of before, the
// word being typed starting after its last dot.
func completeAt(t *testing.T, before, after string) map[string]string {
	t.Helper()
	e, err := godemirror.NewEditor("monospace", 13)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetState(godemirror.EditorStateConfig{Doc: before + after}); err != nil {
		t.Fatal(err)
	}
	from := strings.LastIndexAny(before, ".\n\t ") + 1
	r := complete(context.Background(), godemirror.CompletionRequest{State: e.State(), Pos: len(before), From: from})
	out := map[string]string{}
	for _, it := range r.Items {
		out[it.Label] = it.Detail
	}
	return out
}

const typedSrc = `package main

import "time"

type Item struct {
	SKU     string
	Updated time.Time
}

func (i *Item) Touch(at time.Time) bool { return true }

func f(items map[string]Item) {
	item := items["a"]
	`

func TestADotOffersTheFieldsAndMethodsOfItsType(t *testing.T) {
	got := completeAt(t, typedSrc+"item.", "\n}\n")
	if got["SKU"] != "string" || got["Updated"] != "time.Time" || got["Touch"] != "Touch(at time.Time) bool" {
		t.Fatalf("offered %v", got)
	}
	if _, ok := got["items"]; ok {
		t.Fatal("a file word was offered after a typed dot")
	}
}

func TestADotWithANameStartedStillTypes(t *testing.T) {
	if got := completeAt(t, typedSrc+"item.Up", "\n}\n"); got["Updated"] != "time.Time" {
		t.Fatalf("offered %v", got)
	}
}

func TestADotOnAPackageFallsBackToTheFilesWords(t *testing.T) {
	got := completeAt(t, typedSrc+"time.", "\n}\n")
	var labels []string
	for l := range got {
		labels = append(labels, l)
	}
	if !slices.Contains(labels, "Item") {
		t.Fatalf("offered %v, want the file's words", labels)
	}
}
