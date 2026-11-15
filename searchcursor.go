package godemirror

import (
	"unicode/utf16"

	"golang.org/x/text/unicode/norm"
)

// searchCursor ports @codemirror/search 6.7.2's SearchCursor, with no
// normalize or test function of its own: text and query are compared as
// their NFKD forms, code unit by UTF-16 code unit, so "é" and "e" with a
// combining accent find each other, and "e" finds the "e" "é" decomposes to.
// A match is reported over the whole characters its first and last code
// units come from.
type searchCursor struct {
	query   []uint16
	partial []partialMatch
}

// partialMatch is a match under way: where it started, and how many code
// units of the query it matched.
type partialMatch struct{ from, index int }

func newSearchCursor(query string) *searchCursor {
	return &searchCursor{query: utf16.Encode([]rune(norm.NFKD.String(query)))}
}

// matchesIn are the places query appears in the document's bytes [from,
// to), in order, none overlapping the one before, as SearchCursor.next
// finds them.
func (s EditorState) matchesIn(query string, from, to int) func(yield func(from, to int) bool) {
	return func(yield func(from, to int) bool) {
		c := newSearchCursor(query)
		if len(c.query) == 0 {
			return
		}
		for at, r := range s.doc[from:to] {
			start := from + at
			end := start + len(string(r))
			for _, code := range utf16.Encode([]rune(norm.NFKD.String(string(r)))) {
				if matchFrom, ok := c.match(code, start); ok {
					c.partial = c.partial[:0]
					if !yield(matchFrom, end) {
						return
					}
					break
				}
			}
		}
	}
}

// match takes the next code unit of the normalized text, from a character
// starting at pos, and reports where the match it completes started.
func (c *searchCursor) match(code uint16, pos int) (from int, ok bool) {
	kept := c.partial[:0]
	for _, p := range c.partial {
		if c.query[p.index] != code {
			continue
		}
		if p.index == len(c.query)-1 {
			from, ok = p.from, true
			continue
		}
		kept = append(kept, partialMatch{p.from, p.index + 1})
	}
	c.partial = kept
	if c.query[0] == code {
		if len(c.query) == 1 {
			from, ok = pos, true
		} else {
			c.partial = append(c.partial, partialMatch{pos, 1})
		}
	}
	return from, ok
}
