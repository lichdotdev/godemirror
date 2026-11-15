package godemirror

//go:generate node internal/cmgen/state.mjs $CODEMIRROR_NODE_MODULES testdata/state.golden.json

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// stateCase is one case of testdata/state.golden.json, which
// internal/cmgen/state.mjs writes from the real @codemirror/state.
type stateCase struct {
	Doc     string
	A, B, C []struct {
		From, To int
		Insert   string
	}
	ASections       string
	Applied         string
	Composed        string
	ComposedApplied string
	Mapped          string
	MappedBefore    string
	Inverted        string
	Restored        string
	MapPos          [][4]*int
	Selection       [][2]int
	Main            int
	Created         goldenSelection
	SelectionMapped goldenSelection
	Insert          string
	ReplacedDoc     string
	ReplacedSel     goldenSelection `json:"replacedSelection"`
}

type goldenSelection struct {
	Ranges [][2]int
	Main   int
}

func selectionOf(s EditorSelection) goldenSelection {
	g := goldenSelection{Main: s.MainIndex()}
	for _, r := range s.Ranges() {
		g.Ranges = append(g.Ranges, [2]int{r.Anchor(), r.Head()})
	}
	return g
}

func (g goldenSelection) equal(o goldenSelection) bool {
	return g.Main == o.Main && slices.Equal(g.Ranges, o.Ranges)
}

func TestChangesAndSelectionsMatchCodeMirror(t *testing.T) {
	data, err := os.ReadFile("testdata/state.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Version string
		Cases   []stateCase
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != "6.7.6" {
		t.Fatalf("goldens from @codemirror/state %s, the port is of 6.7.6", golden.Version)
	}
	e := newTestEditor(t, EditorStateConfig{})
	for i, c := range golden.Cases {
		specs := func(raw []struct {
			From, To int
			Insert   string
		}) []ChangeSpec {
			var s []ChangeSpec
			for _, r := range raw {
				s = append(s, ChangeSpec{From: r.From, To: r.To, Insert: r.Insert})
			}
			return s
		}
		a := mustChanges(t, specs(c.A), len(c.Doc))
		b := mustChanges(t, specs(c.B), len(c.Doc))
		cc := mustChanges(t, specs(c.C), a.NewLen())
		for what, got := range map[string][2]string{
			"sections":         {a.String(), c.ASections},
			"applied":          {a.apply(c.Doc), c.Applied},
			"composed":         {a.compose(cc).String(), c.Composed},
			"composed applied": {a.compose(cc).apply(c.Doc), c.ComposedApplied},
			"mapped":           {a.mapOnto(b).String(), c.Mapped},
			"mapped before":    {a.mapOntoBefore(b).String(), c.MappedBefore},
			"inverted":         {a.invert(c.Doc).String(), c.Inverted},
			"restored":         {a.invert(c.Doc).apply(a.apply(c.Doc)), c.Restored},
		} {
			if got[0] != got[1] {
				t.Errorf("case %d (%q, %v): %s %q, CodeMirror %q", i, c.Doc, c.A, what, got[0], got[1])
			}
		}
		checkMapPos(t, i, a, c.MapPos)
		var ranges []SelectionRange
		for _, r := range c.Selection {
			ranges = append(ranges, Range(r[0], r[1]))
		}
		sel := NewSelection(ranges, c.Main)
		if got := selectionOf(sel); !got.equal(c.Created) {
			t.Errorf("case %d: selection %v made %v, CodeMirror %v", i, c.Selection, got, c.Created)
		}
		if got := selectionOf(sel.Map(a)); !got.equal(c.SelectionMapped) {
			t.Errorf("case %d: selection mapped to %v, CodeMirror %v", i, got, c.SelectionMapped)
		}
		if err := e.SetState(EditorStateConfig{Doc: c.Doc, Selection: sel, AllowMultipleSelections: true}); err != nil {
			t.Fatal(err)
		}
		dispatch(t, e, e.State().ReplaceSelection(c.Insert))
		if s := e.State(); s.Doc() != c.ReplacedDoc || !selectionOf(s.Selection()).equal(c.ReplacedSel) {
			t.Errorf("case %d: replacing the selection with %q gave %q %v, CodeMirror %q %v",
				i, c.Insert, s.Doc(), selectionOf(s.Selection()), c.ReplacedDoc, c.ReplacedSel)
		}
	}
}

// checkMapPos checks every position of a, with assoc -1 then 1, in each mode
// as the goldens list them: simple, then TrackDel, TrackBefore, TrackAfter.
func checkMapPos(t *testing.T, i int, a ChangeSet, want [][4]*int) {
	t.Helper()
	modes := []MapMode{MapTrackDel, MapTrackBefore, MapTrackAfter}
	for k, w := range want {
		pos, assoc := k/2, k%2*2-1
		if got := a.MapPos(pos, assoc); w[0] == nil || got != *w[0] {
			t.Errorf("case %d: %s maps %d (assoc %d) to %d, CodeMirror %v", i, a, pos, assoc, got, w[0])
		}
		for m, mode := range modes {
			got, ok := a.MapPosTrack(pos, assoc, mode)
			if exp := w[m+1]; ok != (exp != nil) || ok && got != *exp {
				t.Errorf("case %d: %s maps %d (assoc %d, mode %d) to %d %v, CodeMirror %v", i, a, pos, assoc, mode, got, ok, exp)
			}
		}
	}
}
