package godemirror

import (
	"regexp"
	"strings"
	"testing"

	"gioui.org/io/key"
)

// The selectNextOccurrence tests are @codemirror/search 6.7.2's
// test/test-selection-match.ts, ported: "|" is a cursor and "<...>" a range.

var rangeMark = regexp.MustCompile(`\||<([^>]*)>`)

func markedState(t *testing.T, marked string) EditorState {
	t.Helper()
	var ranges []SelectionRange
	var doc strings.Builder
	last := 0
	for _, m := range rangeMark.FindAllStringSubmatchIndex(marked, -1) {
		doc.WriteString(marked[last:m[0]])
		from := doc.Len()
		if m[2] >= 0 {
			doc.WriteString(marked[m[2]:m[3]])
		}
		ranges = append(ranges, Range(from, doc.Len()))
		last = m[1]
	}
	doc.WriteString(marked[last:])
	s, err := newEditorState(EditorStateConfig{Doc: doc.String(), Selection: NewSelection(ranges, 0), AllowMultipleSelections: true})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func marked(s EditorState) string {
	doc := s.doc
	for i := len(s.sel.ranges) - 1; i >= 0; i-- {
		r := s.sel.ranges[i]
		if r.Empty() {
			doc = doc[:r.from] + "|" + doc[r.from:]
		} else {
			doc = doc[:r.from] + "<" + doc[r.from:r.to] + ">" + doc[r.to:]
		}
	}
	return doc
}

func TestSelectNextOccurrence(t *testing.T) {
	for _, c := range [][2]string{
		// expands to the surrounding word
		{"one| two", "<one> two"},
		{"|one two", "<one> two"},
		{"o|ne two", "<one> two"},
		{"one |two", "one <two>"},
		// selects the next occurrence
		{"<one> one one", "<one> <one> one"},
		{"<one> <one> two one", "<one> <one> two <one>"},
		{"one <one> one", "one <one> <one>"},
		{"one <one> <one>", "<one> <one> <one>"},
		{"<one> <one> <one>", "<one> <one> <one>"},
		// matches full words
		{"<one> two onetwo one", "<one> two onetwo <one>"},
		// matches subwords if a subword is selected
		{"<one>two onethree", "<one>two <one>three"},
		{"<one>two <one> onethree", "<one>two <one> <one>three"},
	} {
		s := markedState(t, c[0])
		if spec, ok := s.selectNextOccurrence(); ok {
			tr, err := s.update(spec)
			if err != nil {
				t.Fatal(err)
			}
			s = tr.State
		}
		if got := marked(s); got != c[1] {
			t.Errorf("%q: got %q, want %q", c[0], got, c[1])
		}
	}
}

func TestCtrlDAndCtrlShiftLSelectOccurrences(t *testing.T) {
	h, events := focusedEditor(t, EditorStateConfig{Doc: "foo bar foo baz foo", AllowMultipleSelections: true}, SingleSelection(1, 1))
	h.press("D", key.ModShortcut)
	h.press("D", key.ModShortcut)
	if got := marked(h.ed.State()); got != "<foo> bar <foo> baz foo" {
		t.Fatalf("Ctrl+D twice left %q", got)
	}
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(4, 7)})
	dispatch(t, h.ed, TransactionSpec{Selection: SingleSelection(0, 3)})
	h.press("L", key.ModShortcut|key.ModShift)
	if got := marked(h.ed.State()); got != "<foo> bar <foo> baz <foo>" || h.ed.State().Selection().MainIndex() != 0 {
		t.Fatalf("Ctrl+Shift+L left %q main %d", got, h.ed.State().Selection().MainIndex())
	}
	if (*events)[len(*events)-1] != "select.search.matches" {
		t.Errorf("events %q, want select.search.matches last", *events)
	}
}

func TestOccurrencesMatchThroughNFKD(t *testing.T) {
	// As CodeMirror's SearchCursor: an "e" finds the "e" a precomposed "é"
	// decomposes to, and the match covers the whole "é"; the two forms of
	// "é", and a ligature and its letters, find each other.
	for _, c := range [][2]string{
		{"<e> é", "<e> <é>"},
		{"<é> e\u0301", "<é> <e\u0301>"},
		{"<fi> ﬁ", "<fi> <ﬁ>"},
	} {
		s := markedState(t, c[0])
		spec, ok := s.selectSelectionMatches()
		if !ok {
			t.Fatalf("%q: nothing selected", c[0])
		}
		tr, err := s.update(spec)
		if err != nil {
			t.Fatal(err)
		}
		if got := marked(tr.State); got != c[1] {
			t.Errorf("%q: got %q, want %q", c[0], got, c[1])
		}
	}
}
