package godemirror

import (
	"cmp"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"unicode/utf8"
)

// SearchQuery is what SetSearch looks for. Text "" clears the search.
type SearchQuery struct {
	Text string
	// CaseSensitive makes a letter match only itself; otherwise Unicode simple
	// case folding applies.
	CaseSensitive bool
	// Regexp reads Text as a Go regular expression (regexp/syntax), matched
	// against one line at a time; otherwise Text is looked for as written.
	Regexp bool
	// WholeWord keeps only the matches that do not cut a word in two: no word
	// character, as the double click reads one (letters, digits and
	// underscore), sits against a match's first or last character when that
	// character is one too.
	WholeWord bool
}

// Match is one match of a search: the runes from From to To (To excluded),
// counted as Pos counts them, so a tab is one. A match never spans lines.
type Match struct{ From, To Pos }

// matcher is a SearchQuery compiled. It is immutable, so the views of a
// DiffView share one.
type matcher struct {
	re        *regexp.Regexp
	wholeWord bool
}

// compileSearch is nil for an empty query.
func compileSearch(q SearchQuery) (*matcher, error) {
	if q.Text == "" {
		return nil, nil
	}
	expr := q.Text
	if q.Regexp {
		// Checked apart so the error does not quote the case flag added below.
		if _, err := syntax.Parse(expr, syntax.Perl); err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
	} else {
		expr = regexp.QuoteMeta(expr)
	}
	if !q.CaseSensitive {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return &matcher{re: re, wholeWord: q.WholeWord}, nil
}

// findAll is every match in lines, in document order.
func (m *matcher) findAll(lines [][]rune) []Match {
	var out []Match
	for i, l := range lines {
		out = m.appendLine(out, i, string(l))
	}
	return out
}

func (m *matcher) appendLine(out []Match, line int, text string) []Match {
	// Matches come in order, so runes are counted from the last one's end.
	var bytes, runes int
	for _, loc := range m.re.FindAllStringIndex(text, -1) {
		from, to := loc[0], loc[1]
		if from == to || m.wholeWord && cutsWord(text, from, to) {
			continue
		}
		start := runes + utf8.RuneCountInString(text[bytes:from])
		end := start + utf8.RuneCountInString(text[from:to])
		bytes, runes = to, end
		out = append(out, Match{Pos{line, start}, Pos{line, end}})
	}
	return out
}

// cutsWord is whether text[from:to] has a word character on both sides of
// either of its ends.
func cutsWord(text string, from, to int) bool {
	before, _ := utf8.DecodeLastRuneInString(text[:from])
	first, _ := utf8.DecodeRuneInString(text[from:to])
	last, _ := utf8.DecodeLastRuneInString(text[from:to])
	after, _ := utf8.DecodeRuneInString(text[to:])
	return isWordRune(before) && isWordRune(first) || isWordRune(last) && isWordRune(after)
}

// SetSearch highlights every match of q in the content shown, and keeps doing
// so as the content changes (SetContent, and UpdateContent once its content is
// on screen). Matches are looked for line by line. An empty q.Text clears the
// search; a Text that is not a valid regexp clears it too and is the only
// error. A theme with no SearchMatch color highlights nothing.
func (v *View) SetSearch(q SearchQuery) error {
	m, err := compileSearch(q)
	v.useSearch(m)
	return err
}

func (v *View) useSearch(m *matcher) {
	v.matcher = m
	v.findMatches()
}

// findMatches runs the search over the document on screen, which is one pass
// over its lines on the calling goroutine.
func (v *View) findMatches() {
	v.matches = nil
	if v.matcher != nil {
		v.matches = v.matcher.findAll(v.doc.source)
	}
}

// Matches are the matches of the search, in document order, in the content on
// screen: between UpdateContent and its swap, the old content's (see
// UpdateContent). It is a copy.
func (v *View) Matches() []Match { return slices.Clone(v.matches) }

// FindNext selects the first match that starts at or after the end of the
// selection (or the first one, with nothing selected), wrapping around the
// document, and scrolls it into view as little as it takes. ok is false when
// there is no match. The match is the current one: lit in Theme.SearchCurrent
// for as long as it stays selected.
func (v *View) FindNext() (m Match, ok bool) {
	if len(v.matches) == 0 {
		return Match{}, false
	}
	var from Pos
	if _, to, ok := v.Selection(); ok {
		from = to
	}
	i, _ := slices.BinarySearchFunc(v.matches, from, func(m Match, p Pos) int { return comparePos(m.From, p) })
	return v.reveal(v.matches[i%len(v.matches)])
}

// FindPrevious is FindNext backwards: the last match that ends at or before
// the start of the selection (with nothing selected, the last one), wrapping
// around the document.
func (v *View) FindPrevious() (m Match, ok bool) {
	if len(v.matches) == 0 {
		return Match{}, false
	}
	var from Pos
	if start, _, ok := v.Selection(); ok {
		from = start
	}
	// The first match ending after from; never equal, so only the place counts.
	i, _ := slices.BinarySearchFunc(v.matches, from, func(m Match, p Pos) int {
		if comparePos(m.To, p) <= 0 {
			return -1
		}
		return 1
	})
	return v.reveal(v.matches[(i-1+len(v.matches))%len(v.matches)])
}

func (v *View) reveal(m Match) (Match, bool) {
	v.Select(m.From, m.To)
	v.target = &scrollTarget{line: m.From.Line, align: AlignNearest, match: &m}
	return m, true
}

func comparePos(a, b Pos) int {
	return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
}

// isCurrentMatch is whether the selection is exactly one of the matches.
func (v *View) isCurrentMatch() bool {
	from, to, ok := v.Selection()
	if !ok {
		return false
	}
	i, found := slices.BinarySearchFunc(v.matches, from, func(m Match, p Pos) int { return comparePos(m.From, p) })
	return found && v.matches[i].To == to
}

// searchSpans are the matches on line as spans, the current one in the
// theme's SearchCurrent. paintSpans paints them with the line's own spans.
func (v *View) searchSpans(line int) []Span {
	i, _ := slices.BinarySearchFunc(v.matches, line, func(m Match, line int) int { return cmp.Compare(m.From.Line, line) })
	var spans []Span
	from, to, selected := v.Selection()
	for ; i < len(v.matches) && v.matches[i].From.Line == line; i++ {
		m := v.matches[i]
		color := v.Theme.SearchMatch
		if selected && m.From == from && m.To == to {
			color = v.Theme.SearchCurrent
		}
		spans = append(spans, Span{From: m.From.Col, To: m.To.Col, Color: color})
	}
	return spans
}
