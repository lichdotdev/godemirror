package godemirror

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The tests in this file are @codemirror/state 6.7.6's test/test-change.ts,
// ported. Its ChangeDesc tests run on change sets here, whose inserted text
// (filler of the right length) plays no part in what they check. The JSON
// round trip is not ported: godemirror does not serialize change sets.

var sectionSpec = regexp.MustCompile(`^(\d+)(?::(\d+))?\s*`)

// mk is the change set spec writes as ChangeSet.String does, its insertions
// filled with "x".
func mk(spec string) ChangeSet {
	var c ChangeSet
	for spec != "" {
		m := sectionSpec.FindStringSubmatch(spec)
		spec = spec[len(m[0]):]
		n, _ := strconv.Atoi(m[1])
		ins := -1
		if m[2] != "" {
			ins, _ = strconv.Atoi(m[2])
		}
		c.sections = append(c.sections, n, ins)
		if ins > 0 {
			for len(c.inserted) < len(c.sections)/2-1 {
				c.inserted = append(c.inserted, "")
			}
			c.inserted = append(c.inserted, strings.Repeat("x", ins))
		}
	}
	return c
}

func TestChangeDescComposition(t *testing.T) {
	for _, c := range [][]string{
		{"5 0:2", "1 2:0 4", "1 2:0 2 0:2"},           // unrelated changes
		{"2 0:2 2", "2 2:0 2", "4"},                   // insertions cancelled by deletions
		{"2 0:2 2", "4 0:3 2", "2 0:5 2"},             // adjacent insertions join
		{"2 5:0", "1 1:0", "1 6:0"},                   // adjacent deletions join
		{"2 2:0 0:3", "5:0", "4:0"},                   // a delete shadows several operations
		{"", "0:8", "8:0", "", ""},                    // empty sets
		{"2 2:2 2:2 2", "1 2:2 2:2 2:2 1", "1 6:6 1"}, // replaces join
		{"1 2:2 2:2 2:2 1", "2 2:2 2:2 2", "1 6:6 1"},
		{"1 2:3 3:2 1", "2 3:1 2", "1 5:3 1"},
	} {
		sets, want := c[:len(c)-1], c[len(c)-1]
		got := mk(sets[0])
		for _, s := range sets[1:] {
			got = got.compose(mk(s))
		}
		if got.String() != want {
			t.Errorf("composing %q = %q, want %q", sets, got, want)
		}
	}
}

func TestChangeDescCompositionPanicsOnInconsistentLengths(t *testing.T) {
	for _, c := range [][2]string{{"2 0:2", "1 0:1"}, {"2 0:2", "30 0:1"}, {"2 2:0 0:3", "7:0"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("composing %q with %q did not panic", c[0], c[1])
				}
			}()
			mk(c[0]).compose(mk(c[1]))
		}()
	}
}

func TestChangeDescMapping(t *testing.T) {
	over := func(a, b, want string) {
		t.Helper()
		if got := mk(a).mapOnto(mk(b)).String(); got != want {
			t.Errorf("%q over %q = %q, want %q", a, b, got, want)
		}
	}
	under := func(a, b, want string) {
		t.Helper()
		if got := mk(a).mapOntoBefore(mk(b)).String(); got != want {
			t.Errorf("%q under %q = %q, want %q", a, b, got, want)
		}
	}
	over("4 0:1", "0:3 4", "7 0:1")       // over an insertion
	over("4 0:1", "2:0 2", "2 0:1")       // over a deletion
	over("2 0:1 2", "2 0:1 2", "3 0:1 2") // orders insertions
	under("2 0:1 2", "2 0:1 2", "2 0:1 3")
	over("2 2:0", "2 1:2 1", "4 1:0") // a deletion over an overlapping replace
	under("2 2:0", "2 1:2 1", "4 1:0")
	over("0:1 2:0 8", "6 1:0 0:5 3", "0:1 2:0 12") // changes after
	over("5:0 2 3:0 2", "4 4:0 4", "6:0 2")        // joins deletions
	under("2 0:1 2", "4:0", "0:1")                 // keeps insertions in deletions
	over("4 0:1 4", "2 4:0 2", "2 0:1 2")
	over("2 2:2 2", "0:2 6", "4 2:2 2") // keeps replacements
	over("2 2:2 2", "3:0 3", "1:2 2")
	over("1 4:4 1", "3 0:2 3", "1 2:4 2 2:0 1")
	over("1 4:4 1", "2 2:0 2", "1 2:4 1")
	over("2 2:2 2", "3 2:0 1", "2 1:2 1")
	over("2:2 2 2:2", "2 2:0 2", "2:2 2:2") // doesn't join replacements
	under("2 2:0 2", "2 2:0 2", "4")        // drops duplicate deletion
	over("2 2:0 2", "2 2:0 2", "4")
	over("1 1:2 1", "1 1:1 1", "2 0:2 1") // overlapping replaces
	under("1 1:2 1", "1 1:1 1", "1 0:2 2")
	over("1 1:2 2", "1 2:1 1", "1 0:2 2")
	over("2 1:2 1", "1 2:1 1", "2 0:2 1")
	over("2:1 1", "1 2:2", "1:1 2")
	over("1 2:1", "2:2 1", "2 1:1")
}

// mapCase is a position, where it should map to (-1 for deleted), and its
// assoc, or the mode it is tracked in: D, A or B.
type mapCase struct {
	from, to int
	opt      any
}

func TestChangeDescMapPos(t *testing.T) {
	modes := map[string]MapMode{"D": MapTrackDel, "A": MapTrackAfter, "B": MapTrackBefore}
	check := func(spec string, cases ...mapCase) {
		t.Helper()
		set := mk(spec)
		for _, c := range cases {
			assoc, got := -1, 0
			if n, ok := c.opt.(int); ok {
				assoc = n
			}
			if m, ok := c.opt.(string); ok {
				p, kept := set.MapPosTrack(c.from, assoc, modes[m])
				got = p
				if !kept {
					got = -1
				}
			} else {
				got = set.MapPos(c.from, assoc)
			}
			if got != c.to {
				t.Errorf("%q maps %d (%v) to %d, want %d", spec, c.from, c.opt, got, c.to)
			}
		}
	}
	check("4 0:2 4", mapCase{0, 0, nil}, mapCase{4, 4, nil}, mapCase{4, 6, 1}, mapCase{5, 7, nil}, mapCase{8, 10, nil})
	check("4 4:0 4", mapCase{0, 0, nil},
		mapCase{4, 4, nil}, mapCase{4, 4, "D"}, mapCase{4, 4, "B"}, mapCase{4, -1, "A"},
		mapCase{5, 4, nil}, mapCase{5, -1, "D"}, mapCase{5, -1, "B"}, mapCase{5, -1, "A"},
		mapCase{7, 4, nil},
		mapCase{8, 4, nil}, mapCase{8, 4, "D"}, mapCase{8, -1, "B"}, mapCase{8, 4, "A"},
		mapCase{9, 5, nil}, mapCase{12, 8, nil})
	check("0:2 2 0:2 2 0:2",
		mapCase{0, 0, nil}, mapCase{0, 2, 1}, mapCase{1, 3, nil}, mapCase{2, 4, nil}, mapCase{2, 6, 1},
		mapCase{3, 7, nil}, mapCase{4, 8, nil}, mapCase{4, 10, 1})
	check("2:0 2 2:0 2 2:0",
		mapCase{0, 0, nil}, mapCase{1, 0, nil}, mapCase{2, 0, nil}, mapCase{3, 1, nil}, mapCase{4, 2, nil},
		mapCase{5, 2, nil}, mapCase{6, 2, nil}, mapCase{7, 3, nil}, mapCase{8, 4, nil}, mapCase{9, 4, nil},
		mapCase{10, 4, nil})
	check("2 0:2 2:0 0:2 2 2:0 0:2",
		mapCase{0, 0, nil}, mapCase{2, 2, nil}, mapCase{2, 4, 1}, mapCase{3, 4, nil}, mapCase{4, 4, nil},
		mapCase{4, 6, 1}, mapCase{5, 7, nil}, mapCase{6, 8, nil}, mapCase{7, 8, nil}, mapCase{8, 8, nil},
		mapCase{8, 10, 1})
	check("2 2:2 2",
		mapCase{2, 2, 1}, mapCase{2, 2, -1}, mapCase{2, 2, "D"}, mapCase{2, 2, "B"}, mapCase{2, -1, "A"},
		mapCase{3, 2, -1}, mapCase{3, 4, 1}, mapCase{3, -1, "D"}, mapCase{3, -1, "B"}, mapCase{3, -1, "A"},
		mapCase{4, 4, 1}, mapCase{4, 4, -1}, mapCase{4, 4, "D"}, mapCase{4, -1, "B"}, mapCase{4, 4, "A"})
	check("0:1 2:2 0:1",
		mapCase{0, 0, -1}, mapCase{0, 1, 1}, mapCase{1, 1, -1}, mapCase{1, 3, 1}, mapCase{2, 3, -1}, mapCase{2, 4, 1})
	check("2:2 2:2", mapCase{2, 2, -1}, mapCase{2, 2, 1})
}

func mustChanges(t *testing.T, specs []ChangeSpec, length int) ChangeSet {
	t.Helper()
	c, err := changeSetOf(specs, length, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ins(from int, text string) ChangeSpec { return ChangeSpec{From: from, To: from, Insert: text} }
func del(from, to int) ChangeSpec          { return ChangeSpec{From: from, To: to} }

func TestChangeSetOfBuildsSections(t *testing.T) {
	for _, c := range []struct {
		specs []ChangeSpec
		want  string
	}{
		{[]ChangeSpec{ins(5, "hi")}, "5 0:2 5"},
		{[]ChangeSpec{del(5, 7)}, "5 2:0 3"},
		{[]ChangeSpec{ins(5, "hi"), ins(5, "ok"), del(0, 3), del(4, 6), ins(8, "boo")}, "3:0 1 1:0 0:4 1:0 2 0:3 2"},
	} {
		if got := mustChanges(t, c.specs, 10).String(); got != c.want {
			t.Errorf("%v = %q, want %q", c.specs, got, c.want)
		}
	}
}

const doc10 = "0123456789"

func TestChangeSetApplyComposeMapInvert(t *testing.T) {
	check := func(what, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", what, got, want)
		}
	}
	check("apply insert", mustChanges(t, []ChangeSpec{ins(2, "ok")}, 10).apply(doc10), "01ok23456789")
	check("apply delete", mustChanges(t, []ChangeSpec{del(1, 9)}, 10).apply(doc10), "09")
	check("apply out of order", mustChanges(t, []ChangeSpec{del(2, 8), ins(1, "hi")}, 10).apply(doc10), "0hi189")
	check("composed", mustChanges(t, []ChangeSpec{ins(8, "ABCD")}, 10).
		compose(mustChanges(t, []ChangeSpec{del(8, 11)}, 14)).apply(doc10), "01234567D89")
	check("composed twice", mustChanges(t, []ChangeSpec{ins(2, "hi"), ins(8, "ok")}, 10).
		compose(mustChanges(t, []ChangeSpec{ins(4, "!"), del(6, 8), ins(12, "?")}, 14)).apply(doc10), "01hi!2367ok?89")
	check("clipped on compose", mustChanges(t, []ChangeSpec{ins(2, "abc"), ins(4, "def")}, 10).
		compose(mustChanges(t, []ChangeSpec{del(4, 8)}, 16)).apply(doc10), "01abef456789")
	set0 := mustChanges(t, []ChangeSpec{ins(5, "hi"), del(8, 10)}, 10)
	set1 := mustChanges(t, []ChangeSpec{ins(10, "ok"), del(6, 7)}, 10)
	check("mapped", set0.compose(set1.mapOnto(set0)).apply(doc10), "01234hi57ok")
	check("inverted", set0.invert(doc10).apply(set0.apply(doc10)), doc10)
}

func TestChangeSetIterates(t *testing.T) {
	set := mustChanges(t, []ChangeSpec{ins(4, "ok"), del(6, 8)}, 10)
	var got []string
	set.IterChanges(func(fromA, toA, fromB, toB int, inserted string) {
		got = append(got, fmt.Sprintf("%d %d %d %d %q", fromA, toA, fromB, toB, inserted))
	})
	if want := `[4 4 4 6 "ok" 6 8 8 8 ""]`; fmt.Sprint(got) != want {
		t.Errorf("changes %v, want %s", got, want)
	}
	got = nil
	set.IterGaps(func(posA, posB, n int) { got = append(got, fmt.Sprint(posA, posB, n)) })
	if want := "[0 0 4 4 6 2 8 8 2]"; fmt.Sprint(got) != want {
		t.Errorf("gaps %v, want %s", got, want)
	}
}

// The random tests below run test-change.ts's own, seeded for repeatable
// failures where CodeMirror's use Math.random.

func rStr(r *rand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + r.IntN(26))
	}
	return string(b)
}

func rChange(r *rand.Rand, n int) ChangeSpec {
	if n == 0 || r.IntN(3) == 0 {
		from := 0
		if n > 0 {
			from = r.IntN(n)
		}
		return ins(from, rStr(r, r.IntN(5)+1))
	}
	from := 0
	if n > 1 {
		from = r.IntN(n - 1)
	}
	c := del(from, min(from+r.IntN(5)+1, n))
	if r.IntN(2) == 0 {
		c.Insert = rStr(r, r.IntN(2)+1)
	}
	return c
}

func rChanges(r *rand.Rand, n, count int) []ChangeSpec {
	specs := make([]ChangeSpec, count)
	for i := range specs {
		specs[i] = rChange(r, n)
	}
	return specs
}

func TestMappingBeforeMatchesMappingTheOtherAfter(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 100 {
		size, count := r.IntN(20), i/10+1
		a, b := mustChanges(t, rChanges(r, size, count), size), mustChanges(t, rChanges(r, size, count), size)
		doc := rStr(r, size)
		ab, ba := a.compose(b.mapOnto(a)).apply(doc), b.compose(a.mapOntoBefore(b)).apply(doc)
		if ab != ba {
			t.Fatalf("a %v then b gives %q, b %v then a %q", a, ab, b, ba)
		}
	}
}

func TestMappingConvergesThroughSeveralChanges(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for i := range 100 {
		size, count := r.IntN(20), i/10+1
		a := mustChanges(t, rChanges(r, size, count), size)
		b := mustChanges(t, rChanges(r, a.NewLen(), count), a.NewLen())
		c := mustChanges(t, rChanges(r, size, count), size)
		cA := c.mapOnto(a)
		cAB, aC, bCA := cA.mapOnto(b), a.mapOntoBefore(c), b.mapOntoBefore(cA)
		doc := rStr(r, size)
		if x, y := a.compose(b).compose(cAB).apply(doc), c.compose(aC).compose(bCA).apply(doc); x != y {
			t.Fatalf("%q != %q", x, y)
		}
	}
}

func TestComposeMatchesTheChangesOneByOne(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	for range 100 {
		size := r.IntN(20)
		doc := rStr(r, size)
		a := mustChanges(t, rChanges(r, size, r.IntN(5)+1), size)
		b := mustChanges(t, rChanges(r, a.NewLen(), r.IntN(6)), a.NewLen())
		if x, y := b.apply(a.apply(doc)), a.compose(b).apply(doc); x != y {
			t.Fatalf("%q != %q", x, y)
		}
	}
}

func TestComposingIsAssociative(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	for range 100 {
		size := r.IntN(20)
		doc := rStr(r, size)
		a := mustChanges(t, rChanges(r, size, r.IntN(5)+1), size)
		b := mustChanges(t, rChanges(r, a.NewLen(), r.IntN(6)), a.NewLen())
		c := mustChanges(t, rChanges(r, b.NewLen(), r.IntN(5)+1), b.NewLen())
		if x, y := a.compose(b).compose(c).apply(doc), a.compose(b.compose(c)).apply(doc); x != y {
			t.Fatalf("%q != %q", x, y)
		}
	}
}

func TestChangeSetsSurviveRandomSequences(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
	for range 50 {
		doc, txt := doc10, doc10
		var all, inv []ChangeSet
		for range 50 {
			change := rChange(r, len(doc))
			txt = txt[:change.From] + change.Insert + txt[change.To:]
			set := mustChanges(t, []ChangeSpec{change}, len(doc))
			all = append(all, set)
			inv = append(inv, set.invert(doc))
			doc = set.apply(doc)
			if doc != txt {
				t.Fatalf("%v gives %q, want %q", change, doc, txt)
			}
		}
		composed := emptyChangeSet(len(doc10))
		for _, s := range all {
			composed = composed.compose(s)
		}
		if got := composed.apply(doc10); got != txt {
			t.Fatalf("composed gives %q, want %q", got, txt)
		}
		if got := composed.invert(doc10).apply(doc); got != doc10 {
			t.Fatalf("composed inverted gives %q", got)
		}
		for i := len(inv) - 1; i >= 0; i-- {
			doc = inv[i].apply(doc)
		}
		if doc != doc10 {
			t.Fatalf("undone one by one gives %q", doc)
		}
	}
}
