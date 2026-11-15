package godemirror

//go:generate node internal/cmgen/commands.mjs $CODEMIRROR_NODE_MODULES testdata/commands.golden.json

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"unicode/utf16"
)

// commandCase is one case of testdata/commands.golden.json, which
// internal/cmgen/commands.mjs writes. Its positions are UTF-16 code units.
type commandCase struct {
	Doc        string
	Multiple   bool
	IndentUnit string
	Sel        unitSelection
	Commands   map[string]*commandResult
	GroupAt    struct {
		Pos, Bias int
		Range     [2]int
	}
	Copied struct {
		Text     string
		Ranges   [][2]int
		Linewise bool
	}
	Paste struct {
		Input        string
		LinewiseCopy *string
		Result       *commandResult
	}
	Input struct {
		From, To, NewSel, Composing int
		Insert                      string
		Result                      *commandResult
	}
}

// unitSelection is a selection as the goldens write it: each range's
// anchor, head, assoc and goal column (-1 for none).
type unitSelection struct {
	Ranges [][4]int
	Main   int
}

type commandResult struct {
	Doc   string
	Sel   unitSelection
	Event string
	Error string
}

// units maps the byte offsets of s that start a character, and its end, to
// UTF-16 code units, and back.
type units struct{ toUnit, toByte map[int]int }

func unitsOf(s string) units {
	u := units{map[int]int{}, map[int]int{}}
	n := 0
	for i, r := range s {
		u.toUnit[i], u.toByte[n] = n, i
		n += utf16.RuneLen(r)
	}
	u.toUnit[len(s)], u.toByte[n] = n, len(s)
	return u
}

func (u units) selection(t *testing.T, g unitSelection) EditorSelection {
	t.Helper()
	var ranges []SelectionRange
	for _, r := range g.Ranges {
		a, h := u.toByte[r[0]], u.toByte[r[1]]
		if a == h {
			ranges = append(ranges, cursorAssoc(a, r[2]))
		} else {
			ranges = append(ranges, rangeWith(a, h, noGoal, r[2]))
		}
	}
	return NewSelection(ranges, g.Main)
}

func (u units) written(s EditorSelection) unitSelection {
	g := unitSelection{Main: s.main}
	for _, r := range s.ranges {
		g.Ranges = append(g.Ranges, [4]int{u.toUnit[r.Anchor()], u.toUnit[r.Head()], r.Assoc(), r.goal})
	}
	return g
}

func loadCommandCases(t *testing.T) []commandCase {
	t.Helper()
	data, err := os.ReadFile("testdata/commands.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Versions map[string]string
		Cases    []commandCase
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	for pkg, want := range map[string]string{
		"@codemirror/state": "6.7.6", "@codemirror/view": "6.43.15",
		"@codemirror/commands": "6.11.1", "@codemirror/language": "6.13.1",
	} {
		if golden.Versions[pkg] != want {
			t.Fatalf("goldens from %s %s, the port is of %s", pkg, golden.Versions[pkg], want)
		}
	}
	return golden.Cases
}

func (c commandCase) state(t *testing.T) EditorState {
	t.Helper()
	s, err := newEditorState(EditorStateConfig{
		Doc: c.Doc, Selection: unitsOf(c.Doc).selection(t, c.Sel),
		AllowMultipleSelections: c.Multiple, IndentUnit: c.IndentUnit,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// checkResult compares what spec makes of s, or ok false, with CodeMirror's.
func checkResult(t *testing.T, what string, s EditorState, spec TransactionSpec, ok bool, want *commandResult) {
	t.Helper()
	if !ok || want == nil {
		if ok != (want != nil) {
			t.Errorf("%s: applies %v, CodeMirror %v", what, ok, want != nil)
		}
		return
	}
	tr, err := s.update(spec)
	if want.Error != "" || err != nil {
		if (err != nil) != (want.Error != "") {
			t.Errorf("%s: error %v, CodeMirror %q", what, err, want.Error)
		}
		return
	}
	got := commandResult{Doc: tr.State.doc, Sel: unitsOf(tr.State.doc).written(tr.State.sel), Event: tr.UserEvent}
	if fmt.Sprint(got) != fmt.Sprint(*want) {
		t.Errorf("%s:\n got  %+v\n want %+v", what, got, *want)
	}
}

func TestCommandsMatchCodeMirror(t *testing.T) {
	for i, c := range loadCommandCases(t) {
		s := c.state(t)
		run := map[string]func() (TransactionSpec, bool){
			"deleteCharBackward":     func() (TransactionSpec, bool) { return s.deleteByChar(false, true) },
			"deleteCharForward":      func() (TransactionSpec, bool) { return s.deleteByChar(true, false) },
			"deleteGroupBackward":    func() (TransactionSpec, bool) { return s.deleteByGroup(false) },
			"deleteGroupForward":     func() (TransactionSpec, bool) { return s.deleteByGroup(true) },
			"insertNewlineAndIndent": s.insertNewlineAndIndent,
			"selectAll":              s.selectAll,
			"cursorDocStart":         func() (TransactionSpec, bool) { return s.cursorDoc(false) },
			"cursorDocEnd":           func() (TransactionSpec, bool) { return s.cursorDoc(true) },
			"selectDocStart":         func() (TransactionSpec, bool) { return s.selectDoc(false) },
			"selectDocEnd":           func() (TransactionSpec, bool) { return s.selectDoc(true) },
		}
		for _, forward := range []bool{false, true} {
			run[fmt.Sprintf("move %v false false", forward)] = func() (TransactionSpec, bool) { return s.cursorByChar(forward) }
			run[fmt.Sprintf("move %v false true", forward)] = func() (TransactionSpec, bool) { return s.selectByChar(forward) }
			run[fmt.Sprintf("move %v true false", forward)] = func() (TransactionSpec, bool) { return s.cursorByGroup(forward) }
			run[fmt.Sprintf("move %v true true", forward)] = func() (TransactionSpec, bool) { return s.selectByGroup(forward) }
		}
		for name, want := range c.Commands {
			f, found := run[name]
			if !found {
				t.Fatalf("no port of %s", name)
			}
			spec, ok := f()
			checkResult(t, fmt.Sprintf("case %d (%q, %v) %s", i, c.Doc, c.Sel, name), s, spec, ok, want)
		}
	}
}

func TestGroupAtAndClipboardMatchCodeMirror(t *testing.T) {
	for i, c := range loadCommandCases(t) {
		s, u := c.state(t), unitsOf(c.Doc)
		g := s.groupAt(u.toByte[c.GroupAt.Pos], c.GroupAt.Bias)
		if got := [2]int{u.toUnit[g.from], u.toUnit[g.to]}; got != c.GroupAt.Range {
			t.Errorf("case %d (%q): groupAt %d bias %d = %v, CodeMirror %v", i, c.Doc, c.GroupAt.Pos, c.GroupAt.Bias, got, c.GroupAt.Range)
		}
		cp := s.copiedRange()
		var ranges [][2]int
		for _, r := range cp.ranges {
			ranges = append(ranges, [2]int{u.toUnit[r.From], u.toUnit[r.To]})
		}
		if cp.text != c.Copied.Text || cp.linewise != c.Copied.Linewise || fmt.Sprint(ranges) != fmt.Sprint(c.Copied.Ranges) {
			t.Errorf("case %d (%q %v): copied %q %v %v, CodeMirror %q %v %v",
				i, c.Doc, c.Sel, cp.text, ranges, cp.linewise, c.Copied.Text, c.Copied.Ranges, c.Copied.Linewise)
		}
		what := fmt.Sprintf("case %d (%q %v) paste %q", i, c.Doc, c.Sel, c.Paste.Input)
		checkResult(t, what, s, s.doPaste(c.Paste.Input, c.Paste.LinewiseCopy), true, c.Paste.Result)
	}
}

func TestTextInputMatchesCodeMirror(t *testing.T) {
	for i, c := range loadCommandCases(t) {
		s, u := c.state(t), unitsOf(c.Doc)
		in := c.Input
		newSel := -1
		if in.NewSel >= 0 {
			after := c.Doc[:u.toByte[in.From]] + in.Insert + c.Doc[u.toByte[in.To]:]
			newSel = unitsOf(after).toByte[in.NewSel]
		}
		change := ChangeSpec{From: u.toByte[in.From], To: u.toByte[in.To], Insert: in.Insert}
		what := fmt.Sprintf("case %d (%q %v) input %+v", i, c.Doc, c.Sel, in)
		checkResult(t, what, s, s.applyDefaultInsert(change, newSel, in.Composing >= 0), true, in.Result)
	}
}
