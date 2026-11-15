package godemirror

//go:generate node internal/cmgen/history.mjs $CODEMIRROR_NODE_MODULES testdata/history.golden.json

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

type historyStep struct {
	Time, Main int
	Kind       string
	Change     *struct {
		From, To int
		Insert   string
	}
	Event   string
	Isolate string
	Sel     [2]int
	OK      bool
	Doc     string
	Sel2    [][2]int
	Depths  [2]int
}

var isolations = map[string]IsolateHistory{"": 0, "before": IsolateBefore, "after": IsolateAfter, "full": IsolateFull}

func TestHistoryMatchesCodeMirror(t *testing.T) {
	data, err := os.ReadFile("testdata/history.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Version string
		Cases   []struct {
			Doc   string
			Steps []historyStep
		}
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != "6.11.1" {
		t.Fatalf("goldens from @codemirror/commands %s, the port is of 6.11.1", golden.Version)
	}
	for i, c := range golden.Cases {
		h := mkHist(t, c.Doc, nil)
		for j, st := range c.Steps {
			at := time.UnixMilli(int64(st.Time))
			ok := true
			switch st.Kind {
			case "type", "receive", "delete":
				h.update(TransactionSpec{Changes: []ChangeSpec{{From: st.Change.From, To: st.Change.To, Insert: st.Change.Insert}},
					UserEvent: st.Event, IsolateHistory: isolations[st.Isolate], SkipHistory: st.Kind == "receive", Time: at})
			case "select":
				h.update(TransactionSpec{Selection: SingleSelection(st.Sel[0], st.Sel[1]), UserEvent: st.Event, Time: at})
			case "isolate":
				h.update(TransactionSpec{IsolateHistory: isolations[st.Isolate], Time: at})
			default:
				var spec TransactionSpec
				switch st.Kind {
				case "undo":
					spec, ok = h.s.hist.popChanges(doneBranch, h.s)
				case "redo":
					spec, ok = h.s.hist.popChanges(undoneBranch, h.s)
				case "undoSelection":
					spec, ok = h.s.hist.popSelectionOf(doneBranch, h.s)
				case "redoSelection":
					spec, ok = h.s.hist.popSelectionOf(undoneBranch, h.s)
				}
				if ok {
					spec.Time = at
					h.update(spec)
				}
			}
			var sel [][2]int
			for _, r := range h.s.sel.ranges {
				sel = append(sel, [2]int{r.Anchor(), r.Head()})
			}
			depths := [2]int{h.s.hist.depth(doneBranch), h.s.hist.depth(undoneBranch)}
			got := fmt.Sprint(h.s.doc, sel, h.s.sel.main, depths)
			want := fmt.Sprint(st.Doc, st.Sel2, st.Main, st.Depths)
			if st.Kind[0] == 'u' || st.Kind[0] == 'r' && st.Kind != "receive" {
				got, want = fmt.Sprint(ok, " ", got), fmt.Sprint(st.OK, " ", want)
			}
			if got != want {
				t.Fatalf("case %d step %d (%s %+v):\n got  %s\n want %s", i, j, st.Kind, st, got, want)
			}
		}
	}
}
