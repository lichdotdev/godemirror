package godemirror

import (
	"context"
	"strings"
	"testing"

	"gioui.org/io/key"
)

// signatureEditor answers SignatureHelp with the parameter the cursor is
// in, counting commas after the last "(", and nothing past its ")".
func signatureEditor(t *testing.T, doc string) (*harness, chan string) {
	t.Helper()
	h, _ := focusedEditor(t, EditorStateConfig{Doc: doc}, SingleSelection(len(doc), len(doc)))
	triggers := make(chan string, 16)
	h.ed.SignatureTriggers = []string{"(", ","}
	h.ed.SignatureHelp = func(_ context.Context, s EditorState, pos int, trigger string) (Signature, bool) {
		triggers <- trigger
		before := s.Doc()[:pos]
		open := strings.LastIndex(before, "(")
		if open < 0 || strings.Contains(before[open:], ")") {
			return Signature{}, false
		}
		return Signature{
			Label:  "f(a int, b string)",
			Params: []SignatureParam{{2, 7}, {9, 17}},
			Active: strings.Count(before[open:], ","),
		}, true
	}
	return h, triggers
}

func TestSignatureHelpFollowsTheCall(t *testing.T) {
	h, triggers := signatureEditor(t, "x := f")
	h.typeText("(")
	h.until("shown", func() bool { return h.ed.signature.sig.Label != "" })
	if got := <-triggers; got != "(" {
		t.Fatalf("asked with trigger %q, want (", got)
	}
	if h.ed.signature.sig.Active != 0 {
		t.Fatalf("active %d, want 0", h.ed.signature.sig.Active)
	}
	h.typeText("1,")
	h.until("on b", func() bool { return h.ed.signature.sig.Active == 1 })
	h.typeText(" \"s\")")
	h.until("closed", func() bool { return !h.ed.signature.open })
}

func TestEscapeClosesTheSignature(t *testing.T) {
	h, _ := signatureEditor(t, "f")
	h.typeText("(")
	h.until("shown", func() bool { return h.ed.signature.sig.Label != "" })
	h.press(key.NameEscape, 0)
	if h.ed.signature.open {
		t.Fatal("Escape left the signature open")
	}
}

func TestTheActiveParameterIsBold(t *testing.T) {
	got := signatureInlines(Signature{Label: "f(a int, b string)", Params: []SignatureParam{{2, 7}, {9, 17}}, Active: 1})
	if len(got) != 3 || got[1].text != "b string" || !got[1].bold || got[0].bold {
		t.Fatalf("inlines %+v", got)
	}
}
