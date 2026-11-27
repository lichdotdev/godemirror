package godemirror

import (
	"context"
	"testing"
	"time"

	"gioui.org/io/key"
)

// completingEditor is a focused editor at the end of doc whose Complete
// answers items, ranged over the word being typed, and counts its calls.
func completingEditor(t *testing.T, doc string, items ...string) (*harness, chan CompletionRequest) {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: doc}, SingleSelection(len(doc), len(doc)))
	asked := make(chan CompletionRequest, 16)
	h.ed.CompletionTriggers = []string{"."}
	h.ed.Complete = func(_ context.Context, req CompletionRequest) CompletionResult {
		asked <- req
		var r CompletionResult
		for _, l := range items {
			r.Items = append(r.Items, CompletionItem{Label: l, Kind: CompletionFunction, From: req.From, To: req.Pos})
		}
		return r
	}
	return h, asked
}

func (h *harness) shownLabels() []string {
	c := &h.ed.completion
	var out []string
	for _, i := range c.shown {
		out = append(out, c.items[i].Label)
	}
	return out
}

func (h *harness) waitForCompletions() {
	h.t.Helper()
	h.until("offered completions", func() bool { return len(h.ed.completion.shown) > 0 })
}

func TestATriggerCharacterAsksForCompletions(t *testing.T) {
	h, asked := completingEditor(t, "restart", "Relaunch", "Requested")
	h.typeText(".")
	h.waitForCompletions()
	req := <-asked
	if req.Trigger != "." || req.Pos != 8 || req.From != 8 || req.Explicit {
		t.Fatalf("asked %+v, want the trigger . at 8", req)
	}
	if got := h.shownLabels(); len(got) != 2 {
		t.Fatalf("shown %v", got)
	}
}

func TestTwoWordCharactersAskForCompletionsAndTypingFilters(t *testing.T) {
	h, asked := completingEditor(t, "x.", "Relaunch", "Requested", "ReadyFile")
	h.typeText("R")
	if h.ed.completion.open {
		t.Fatal("one character opened the completions")
	}
	h.typeText("e")
	h.waitForCompletions()
	if req := <-asked; req.From != 2 || req.Pos != 4 || req.Trigger != "" {
		t.Fatalf("asked %+v, want the word Re from 2", req)
	}
	h.typeText("q")
	if got := h.shownLabels(); len(got) != 1 || got[0] != "Requested" {
		t.Fatalf("after Req: %v, want Requested", got)
	}
	if len(asked) != 0 {
		t.Fatal("a complete result was asked for again while typing")
	}
	h.press(key.NameDeleteBackward, 0)
	if got := h.shownLabels(); len(got) != 3 {
		t.Fatalf("after deleting back to Re: %v", got)
	}
}

func TestArrowsChooseAndEnterTakesTheItem(t *testing.T) {
	h, _ := completingEditor(t, "x.", "Relaunch", "Requested", "ReadyFile")
	h.typeText("Re")
	h.waitForCompletions()
	h.press(key.NameDownArrow, 0)
	h.press(key.NameDownArrow, 0)
	h.press(key.NameUpArrow, 0)
	h.press(key.NameReturn, 0)
	if got := h.ed.State().Doc(); got != "x.Requested" {
		t.Fatalf("doc %q, want x.Requested", got)
	}
	if _, hd := h.mainRange(); hd != 11 || h.ed.completion.open {
		t.Fatalf("cursor at %d, menu open %v; want 11 and closed", hd, h.ed.completion.open)
	}
}

func TestTabTakesAndEscapeCloses(t *testing.T) {
	h, _ := completingEditor(t, "x.", "Relaunch")
	h.typeText("Re")
	h.waitForCompletions()
	h.press(key.NameEscape, 0)
	if h.ed.completion.open || h.ed.State().Doc() != "x.Re" {
		t.Fatalf("Escape left %q, open %v", h.ed.State().Doc(), h.ed.completion.open)
	}
	h.press(key.NameSpace, key.ModShortcut)
	h.waitForCompletions()
	h.press(key.NameTab, 0)
	if got := h.ed.State().Doc(); got != "x.Relaunch" {
		t.Fatalf("Tab left %q, want x.Relaunch", got)
	}
}

func TestCtrlSpaceAsksExplicitly(t *testing.T) {
	h, asked := completingEditor(t, "a ", "alpha")
	h.press(key.NameSpace, key.ModShortcut)
	h.waitForCompletions()
	if req := <-asked; !req.Explicit || req.From != 2 {
		t.Fatalf("asked %+v, want an explicit request from 2", req)
	}
}

func TestTypingPastTheWordCloses(t *testing.T) {
	h, _ := completingEditor(t, "x.", "Relaunch")
	h.typeText("Re")
	h.waitForCompletions()
	h.typeText(" ")
	if h.ed.completion.open {
		t.Fatal("a space left the completions open")
	}
	h.typeText("Re")
	h.waitForCompletions()
	h.press(key.NameLeftArrow, 0)
	h.press(key.NameLeftArrow, 0)
	h.press(key.NameLeftArrow, 0)
	if h.ed.completion.open {
		t.Fatal("moving out of the word left the completions open")
	}
}

func TestAnIncompleteResultIsAskedAgain(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: ""}, SingleSelection(0, 0))
	asked := make(chan string, 16)
	h.ed.Complete = func(_ context.Context, req CompletionRequest) CompletionResult {
		typed := req.State.Doc()[req.From:req.Pos]
		asked <- typed
		return CompletionResult{Incomplete: true, Items: []CompletionItem{{Label: typed + "z", From: req.From, To: req.Pos}}}
	}
	h.typeText("ab")
	h.waitForCompletions()
	h.typeText("c")
	h.until("asked again", func() bool { return len(h.shownLabels()) == 1 && h.shownLabels()[0] == "abcz" })
	if first, second := <-asked, <-asked; first != "ab" || second != "abc" {
		t.Fatalf("asked %q then %q", first, second)
	}
}

func TestTheChosenItemIsResolved(t *testing.T) {
	h, _ := completingEditor(t, "x.", "Relaunch", "Requested")
	h.ed.ResolveCompletion = func(_ context.Context, it CompletionItem) CompletionItem {
		it.Documentation = "docs of " + it.Label
		return it
	}
	h.typeText("Re")
	h.waitForCompletions()
	h.until("resolved", func() bool { return h.ed.completion.items[0].Documentation == "docs of Relaunch" })
	h.press(key.NameDownArrow, 0)
	h.until("resolved", func() bool { return h.ed.completion.items[1].Documentation == "docs of Requested" })
}

func TestAnItemReplacesItsOwnRange(t *testing.T) {
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "fmt.Pr"}, SingleSelection(6, 6))
	h.ed.Complete = func(_ context.Context, req CompletionRequest) CompletionResult {
		return CompletionResult{Items: []CompletionItem{{Label: "Println", Insert: "Println", From: 4, To: 6}}}
	}
	h.press(key.NameSpace, key.ModShortcut)
	h.waitForCompletions()
	h.typeText("i")
	h.press(key.NameReturn, 0)
	if got := h.ed.State().Doc(); got != "fmt.Println" {
		t.Fatalf("doc %q, want fmt.Println: the range grows over what was typed after", got)
	}
}

func TestABurstOfTypingAsksOnce(t *testing.T) {
	h, asked := completingEditor(t, "x.", "Requested")
	for _, s := range []string{"R", "e", "q", "u"} {
		h.typeText(s)
	}
	h.waitForCompletions()
	if req := <-asked; req.Pos != 6 {
		t.Fatalf("asked at %d, want once at 6, after the burst", req.Pos)
	}
	h.advance(time.Second)
	if len(asked) != 0 {
		t.Fatalf("%d more requests for one burst of typing", len(asked))
	}
}

// lateEditor is a focused editor whose Complete answers an incomplete
// result from answers, keyed by the word typed, only once release gets a
// value.
func lateEditor(t *testing.T, answers map[string][]string) (*harness, chan string, chan struct{}) {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: "x."}, SingleSelection(2, 2))
	asked, release := make(chan string, 16), make(chan struct{})
	h.ed.Complete = func(ctx context.Context, req CompletionRequest) CompletionResult {
		typed := req.State.Doc()[req.From:req.Pos]
		asked <- typed
		select {
		case <-release:
		case <-ctx.Done():
			return CompletionResult{}
		}
		r := CompletionResult{Incomplete: true}
		for _, l := range answers[typed] {
			r.Items = append(r.Items, CompletionItem{Label: l, From: req.From, To: req.Pos})
		}
		return r
	}
	return h, asked, release
}

func TestTheMenuStaysWhileALateAnswerComes(t *testing.T) {
	h, asked, release := lateEditor(t, map[string][]string{
		"Re":  {"Relaunch", "Requested", "ReadyFile"},
		"Req": {"Requeue", "Requested"},
	})
	h.typeText("R")
	h.typeText("e")
	h.until("asked", func() bool { return len(asked) == 1 })
	release <- struct{}{}
	h.waitForCompletions()
	h.press(key.NameDownArrow, 0)
	h.typeText("q")
	for range 40 {
		if got := h.shownLabels(); !h.ed.completion.open || len(got) != 1 || got[0] != "Requested" {
			t.Fatalf("while the answer for Req is on its way the menu shows %v, open %v", got, h.ed.completion.open)
		}
		h.advance(10 * time.Millisecond)
	}
	if got := <-asked; got != "Re" {
		t.Fatalf("first request for %q", got)
	}
	if got := <-asked; got != "Req" {
		t.Fatalf("asked again for %q, want Req", got)
	}
	release <- struct{}{}
	h.until("swapped", func() bool { return len(h.shownLabels()) == 2 })
	c := &h.ed.completion
	if got := c.items[c.shown[c.sel]].Label; got != "Requested" {
		t.Fatalf("chosen %q after the answer, want Requested kept", got)
	}
}

func TestATriggerAsksAtOnce(t *testing.T) {
	h, asked := completingEditor(t, "fmt", "Println")
	h.typeText(".")
	h.frame()
	select {
	case req := <-asked:
		if req.Trigger != "." {
			t.Fatalf("asked %+v", req)
		}
	case <-time.After(time.Second):
		t.Fatal("a trigger character waited for a pause")
	}
}
