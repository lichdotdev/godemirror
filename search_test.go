package godemirror

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// matchesOf runs q over text in a fresh view and returns what it matched.
func matchesOf(t *testing.T, text string, q SearchQuery) []Match {
	t.Helper()
	h := newHarness(t, text)
	if err := h.v.SetSearch(q); err != nil {
		t.Fatal(err)
	}
	return h.v.Matches()
}

func at(line, from, to int) Match {
	return Match{From: Pos{line, from}, To: Pos{line, to}}
}

func TestALiteralSearchMatchesEveryOccurrenceInOrder(t *testing.T) {
	got := matchesOf(t, "foo bar foo\nnothing\nfoofoo", SearchQuery{Text: "foo", CaseSensitive: true})
	want := []Match{at(0, 0, 3), at(0, 8, 11), at(2, 0, 3), at(2, 3, 6)}
	if !slices.Equal(got, want) {
		t.Fatalf("Matches() = %v, want %v", got, want)
	}
}

func TestALiteralIsNotARegexp(t *testing.T) {
	got := matchesOf(t, "a.c abc", SearchQuery{Text: "a.c", CaseSensitive: true})
	if want := []Match{at(0, 0, 3)}; !slices.Equal(got, want) {
		t.Fatalf("Matches() = %v, want %v: the dot matched any character", got, want)
	}
}

func TestSearchIgnoresCaseUnlessAskedNotTo(t *testing.T) {
	text := "Foo FOO foo"
	if got := matchesOf(t, text, SearchQuery{Text: "foo"}); len(got) != 3 {
		t.Fatalf("case-insensitive found %v, want 3 matches", got)
	}
	got := matchesOf(t, text, SearchQuery{Text: "foo", CaseSensitive: true})
	if want := []Match{at(0, 8, 11)}; !slices.Equal(got, want) {
		t.Fatalf("case-sensitive found %v, want %v", got, want)
	}
}

func TestARegexpSearchMatchesPerLineAndNeverAcrossALineEnd(t *testing.T) {
	got := matchesOf(t, "ab12 x\n34cd\nab\n12", SearchQuery{Text: `\d+`, Regexp: true})
	want := []Match{at(0, 2, 4), at(1, 0, 2), at(3, 0, 2)}
	if !slices.Equal(got, want) {
		t.Fatalf("Matches() = %v, want %v", got, want)
	}
	if got := matchesOf(t, "ab\ncd", SearchQuery{Text: `b\sc`, Regexp: true}); len(got) != 0 {
		t.Fatalf("a regexp matched across a line end: %v", got)
	}
}

func TestARegexpIsCaseInsensitiveByDefaultAndKeepsItsOwnFlags(t *testing.T) {
	if got := matchesOf(t, "Foo foo", SearchQuery{Text: `f(o+)`, Regexp: true}); len(got) != 2 {
		t.Fatalf("found %v, want 2 matches", got)
	}
	got := matchesOf(t, "Foo foo", SearchQuery{Text: `(?-i)f(o+)`, Regexp: true})
	if want := []Match{at(0, 4, 7)}; !slices.Equal(got, want) {
		t.Fatalf("found %v, want %v", got, want)
	}
}

func TestWholeWordKeepsOnlyMatchesBoundedByNonWordCharacters(t *testing.T) {
	text := "cat concat cat_x (cat) cat"
	got := matchesOf(t, text, SearchQuery{Text: "cat", WholeWord: true})
	want := []Match{at(0, 0, 3), at(0, 18, 21), at(0, 23, 26)}
	if !slices.Equal(got, want) {
		t.Fatalf("Matches() = %v, want %v", got, want)
	}
	if got := matchesOf(t, "foo(x)", SearchQuery{Text: "foo(", WholeWord: true}); len(got) != 1 {
		t.Fatalf("a query ending in punctuation found %v, want its one match: there is no word to cut", got)
	}
}

func TestAnEmptyMatchIsNotAMatch(t *testing.T) {
	if got := matchesOf(t, "abc", SearchQuery{Text: `x*`, Regexp: true}); len(got) != 0 {
		t.Fatalf("an empty regexp match was reported: %v", got)
	}
}

func TestABadRegexpIsAnErrorAndShowsNothing(t *testing.T) {
	h := newHarness(t, "a(b")
	if err := h.v.SetSearch(SearchQuery{Text: "b"}); err != nil {
		t.Fatal(err)
	}
	err := h.v.SetSearch(SearchQuery{Text: "(", Regexp: true})
	if err == nil {
		t.Fatal("an unclosed group was accepted")
	}
	if strings.Contains(err.Error(), "(?i)") {
		t.Fatalf("the error talks about the case flag the library added: %v", err)
	}
	if got := h.v.Matches(); len(got) != 0 {
		t.Fatalf("Matches() = %v after a bad regexp, want none", got)
	}
	h.frame()
}

func TestAnEmptyQueryClearsTheSearch(t *testing.T) {
	h := newHarness(t, "foo")
	h.v.SetSearch(SearchQuery{Text: "foo"})
	if err := h.v.SetSearch(SearchQuery{}); err != nil || len(h.v.Matches()) != 0 {
		t.Fatalf("SetSearch({}) = %v, with matches %v; want none", err, h.v.Matches())
	}
	if _, ok := h.v.FindNext(); ok {
		t.Fatal("FindNext found a match with no search")
	}
}

func TestMatchColumnsAreRunesAndTheirCellsFollowTabs(t *testing.T) {
	h := newHarness(t, "\tπ foo\n\t\tfoo")
	h.v.SetSearch(SearchQuery{Text: "foo"})
	want := []Match{at(0, 3, 6), at(1, 2, 5)}
	if got := h.v.Matches(); !slices.Equal(got, want) {
		t.Fatalf("Matches() = %v, want %v", got, want)
	}
	for _, c := range []struct{ line, from, to int }{{0, tabSize + 2, tabSize + 5}, {1, 2 * tabSize, 2*tabSize + 3}} {
		spans := h.v.searchSpans(c.line)
		if len(spans) != 1 {
			t.Fatalf("line %d: %d search spans, want 1", c.line, len(spans))
		}
		if from, to := h.v.spanCells(c.line, spans[0]); from != c.from || to != c.to {
			t.Fatalf("line %d: match covers cells [%d,%d), want [%d,%d)", c.line, from, to, c.from, c.to)
		}
	}
}

func TestFindNextSelectsTheMatchAfterTheSelectionAndWraps(t *testing.T) {
	h := newHarness(t, "foo\nbar foo\nfoo")
	h.v.SetSearch(SearchQuery{Text: "foo"})
	var got []Match
	for range 4 {
		m, ok := h.v.FindNext()
		if !ok {
			t.Fatal("FindNext found nothing")
		}
		from, to, selected := h.v.Selection()
		if !selected || from != m.From || to != m.To {
			t.Fatalf("selection = %v-%v, %v; want the match %v", from, to, selected, m)
		}
		got = append(got, m)
	}
	want := []Match{at(0, 0, 3), at(1, 4, 7), at(2, 0, 3), at(0, 0, 3)}
	if !slices.Equal(got, want) {
		t.Fatalf("FindNext walked %v, want %v", got, want)
	}
}

func TestFindNextStartsFromTheSelectionsEnd(t *testing.T) {
	h := newHarness(t, "foo foo foo")
	h.v.SetSearch(SearchQuery{Text: "foo"})
	h.v.Select(Pos{0, 2}, Pos{0, 5})
	if m, _ := h.v.FindNext(); m != at(0, 8, 11) {
		t.Fatalf("FindNext = %v, want the match after the selection's end", m)
	}
}

func TestFindPreviousWalksBackwardsAndWraps(t *testing.T) {
	h := newHarness(t, "foo\nbar foo\nfoo")
	h.v.SetSearch(SearchQuery{Text: "foo"})
	var got []Match
	for range 4 {
		m, ok := h.v.FindPrevious()
		if !ok {
			t.Fatal("FindPrevious found nothing")
		}
		got = append(got, m)
	}
	want := []Match{at(2, 0, 3), at(1, 4, 7), at(0, 0, 3), at(2, 0, 3)}
	if !slices.Equal(got, want) {
		t.Fatalf("FindPrevious walked %v, want %v", got, want)
	}
}

func TestFindScrollsTheMatchIntoView(t *testing.T) {
	h := newHarness(t, strings.Repeat("filler\n", 200)+"needle")
	h.v.SetSearch(SearchQuery{Text: "needle"})
	if _, ok := h.v.FindNext(); !ok {
		t.Fatal("FindNext found nothing")
	}
	h.frame()
	ch := h.v.r.CellH()
	if top, bottom := h.v.scroll.Y, h.v.scroll.Y+h.size.Y; 200*ch < top || 201*ch > bottom {
		t.Fatalf("view shows y [%d,%d), the match is at [%d,%d)", top, bottom, 200*ch, 201*ch)
	}
}

func TestFindPreviousScrollsUpToTheMatch(t *testing.T) {
	h := newHarness(t, "needle\n"+strings.Repeat("filler\n", 199)+"needle")
	h.v.SetSearch(SearchQuery{Text: "needle"})
	h.v.Select(Pos{200, 0}, Pos{200, 6})
	h.v.scroll.Y = 190 * h.v.r.CellH()
	if _, ok := h.v.FindPrevious(); !ok {
		t.Fatal("FindPrevious found nothing")
	}
	h.frame()
	if h.v.scroll.Y != 0 {
		t.Fatalf("scroll = %d after FindPrevious, want the match at the top in view", h.v.scroll.Y)
	}
}

func TestTheSelectedMatchIsTheCurrentOne(t *testing.T) {
	h := newHarness(t, "foo foo")
	h.v.SetSearch(SearchQuery{Text: "foo"})
	h.v.FindNext()
	spans := h.v.searchSpans(0)
	if len(spans) != 2 || spans[0].Color != DarkTheme.SearchCurrent || spans[1].Color != DarkTheme.SearchMatch {
		t.Fatalf("search spans = %+v, want the current match then a plain one", spans)
	}
	h.v.Select(Pos{0, 0}, Pos{0, 2})
	if spans := h.v.searchSpans(0); spans[0].Color != DarkTheme.SearchMatch {
		t.Fatal("a selection that is not a match is lit as the current match")
	}
}

func TestSearchFollowsSetContent(t *testing.T) {
	h := newHarness(t, "foo")
	h.v.SetSearch(SearchQuery{Text: "foo"})
	h.v.SetContent(Content{Text: "a\nb foo foo"})
	if want := []Match{at(1, 2, 5), at(1, 6, 9)}; !slices.Equal(h.v.Matches(), want) {
		t.Fatalf("Matches() = %v, want %v", h.v.Matches(), want)
	}
	waitForHighlight(h)
	if want := []Match{at(1, 2, 5), at(1, 6, 9)}; !slices.Equal(h.v.Matches(), want) {
		t.Fatalf("Matches() = %v once highlighted, want %v", h.v.Matches(), want)
	}
}

func TestSearchFollowsUpdateContentOnceItIsOnScreen(t *testing.T) {
	h := newHarness(t, "foo")
	h.v.SetSearch(SearchQuery{Text: "foo"})
	h.v.UpdateContent(Content{Filename: "x.go", Text: "// foo\nfoo foo"})
	if want := []Match{at(0, 0, 3)}; !slices.Equal(h.v.Matches(), want) {
		t.Fatalf("Matches() = %v before the swap, want the content on screen's %v", h.v.Matches(), want)
	}
	waitForHighlight(h)
	if want := []Match{at(0, 3, 6), at(1, 0, 3), at(1, 4, 7)}; !slices.Equal(h.v.Matches(), want) {
		t.Fatalf("Matches() = %v after the swap, want %v", h.v.Matches(), want)
	}
}

func TestSearchFindsEveryMatchOfALargeFile(t *testing.T) {
	const lines = 13000
	h := newHarness(t, "")
	h.v.SetContent(Content{Text: strings.Repeat("func (v *View) paintRows(gtx layout.Context) { return foo(bar, 42) }\n", lines)})
	start := time.Now()
	if err := h.v.SetSearch(SearchQuery{Text: "foo"}); err != nil {
		t.Fatal(err)
	}
	t.Logf("SetSearch over %d lines: %v", lines, time.Since(start))
	if got := len(h.v.Matches()); got != lines {
		t.Fatalf("%d matches, want %d", got, lines)
	}
	waitForHighlight(h)
}

func TestDiffViewSearchesEveryViewItHas(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetDocs("main.go", "a\nfoo\nc", "a\nFOO\nfoo\nc")
	if err := d.SetSearch(SearchQuery{Text: "foo"}); err != nil {
		t.Fatal(err)
	}
	if got := len(d.unified.Matches()); got != 2 {
		t.Fatalf("unified has %d matches, want 2", got)
	}
	if l, r := len(d.split.Left.Matches()), len(d.split.Right.Matches()); l != 1 || r != 2 {
		t.Fatalf("split sides have %d and %d matches, want 1 and 2", l, r)
	}
	if err := d.SetSearch(SearchQuery{Text: "(", Regexp: true}); err == nil {
		t.Fatal("a bad regexp was accepted")
	}
	if len(d.unified.Matches())+len(d.split.Left.Matches())+len(d.split.Right.Matches()) != 0 {
		t.Fatal("a bad regexp left matches on a view")
	}
	h.frame()
}

func TestDiffViewFindActsOnTheViewOnScreen(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetDocs("main.go", "a\nfoo\nc", "a\nFOO\nfoo\nc")
	d.SetSearch(SearchQuery{Text: "foo"})
	m, ok := d.FindNext()
	if !ok || m != at(1, 0, 3) {
		t.Fatalf("unified FindNext = %v, %v; want the added FOO", m, ok)
	}
	if r, ok := d.SelectedRange(); !ok || r != (DiffRange{NewFrom: 2, NewTo: 2}) {
		t.Fatalf("SelectedRange() = %+v, %v; want the new line 2", r, ok)
	}
	d.Split = true
	h.v = d.split.Right
	h.frame()
	if m, ok := d.FindNext(); !ok || m != at(1, 0, 3) {
		t.Fatalf("split FindNext = %v, %v", m, ok)
	}
	if _, _, ok := d.split.Right.Selection(); !ok {
		t.Fatal("with no selection on a side, the right side should have taken the search")
	}
	m, _ = d.FindNext()
	if m != at(2, 0, 3) {
		t.Fatalf("second FindNext on the right side = %v, want its second match", m)
	}
	d.FindPrevious()
	if from, _, _ := d.split.Right.Selection(); from.Line != 1 {
		t.Fatalf("FindPrevious left the selection at %v, want line 1", from)
	}
}

// matchOnScreen reports whether the cells of m lie inside the code area as
// the view is scrolled now.
func matchOnScreen(h *harness, m Match) bool {
	cw := h.v.r.CellW()
	g := h.v.geometry(h.size, cw)
	from, to := h.v.cellPos(m.From).col*cw, h.v.cellPos(m.To).col*cw
	return from >= h.v.scroll.X && to <= h.v.scroll.X+h.size.X-g.textX
}

func TestFindNextScrollsSidewaysToAMatchPastTheEdge(t *testing.T) {
	line := strings.Repeat("x", 300) + "needle" + strings.Repeat("x", 300)
	h := newHarness(t, "needle\n"+line)
	if err := h.v.SetSearch(SearchQuery{Text: "needle"}); err != nil {
		t.Fatal(err)
	}
	h.v.FindNext()
	h.frame()
	m, _ := h.v.FindNext()
	h.frame()
	if !matchOnScreen(h, m) {
		t.Fatalf("match %v is off screen at scroll %v", m, h.v.scroll)
	}
	m, _ = h.v.FindNext()
	h.frame()
	if m.From.Line != 0 || h.v.scroll.X != 0 {
		t.Fatalf("wrapping back to %v left the view scrolled to %v", m, h.v.scroll)
	}
}

func TestFindNextDoesNotScrollAWrappedViewSideways(t *testing.T) {
	h := newHarness(t, strings.Repeat("x", 300)+"needle")
	h.v.Wrap = true
	h.frame()
	if err := h.v.SetSearch(SearchQuery{Text: "needle"}); err != nil {
		t.Fatal(err)
	}
	h.v.FindNext()
	h.frame()
	if h.v.scroll.X != 0 {
		t.Fatalf("a wrapped view scrolled sideways to %d", h.v.scroll.X)
	}
}

func TestDiffViewMatchesAreThoseOfTheViewOnScreen(t *testing.T) {
	h, d := newDiffHarness(t, DiffOptions{})
	d.SetDocs("main.go", "a\nold x\nc", "a\nnew x x\nc")
	h.frame()
	if err := d.SetSearch(SearchQuery{Text: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := len(d.Matches()); got != 3 {
		t.Fatalf("unified: %d matches, want 3", got)
	}
	d.Split = true
	h.frame()
	if got := len(d.Matches()); got != 2 {
		t.Fatalf("side by side: %d matches, want the right side's 2", got)
	}
}
