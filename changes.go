package godemirror

import (
	"strings"
	"unicode"
)

// changedSpans marks what changed between a deleted line and the added line
// it pairs with, word by word: the words of old that are gone and the words
// of new that are new, in rune columns. Two lines with no word in common get
// none, since the whole line is the change.
func changedSpans(old, new string) (oldSpans, newSpans []Span) {
	oldWords, newWords := words(old), words(new)
	shared := false
	oldCol, newCol := 0, 0
	for _, e := range diffLines(oldWords, newWords, maxDiffEdits) {
		n := len([]rune(e.text))
		switch e.op {
		case opEqual:
			shared = shared || strings.TrimSpace(e.text) != ""
			oldCol += n
			newCol += n
		case opDelete:
			oldSpans = widenSpan(oldSpans, oldCol, oldCol+n)
			oldCol += n
		case opInsert:
			newSpans = widenSpan(newSpans, newCol, newCol+n)
			newCol += n
		}
	}
	if !shared {
		return nil, nil
	}
	return oldSpans, newSpans
}

// widenSpan adds [from, to) to spans, joining it to the last span it touches.
func widenSpan(spans []Span, from, to int) []Span {
	if n := len(spans); n > 0 && spans[n-1].To == from {
		spans[n-1].To = to
		return spans
	}
	return append(spans, Span{From: from, To: to})
}

// words cuts a line into runs of word characters, runs of spaces, and single
// other characters, which is what a change is measured in.
func words(line string) []string {
	var out []string
	runes := []rune(line)
	for i := 0; i < len(runes); {
		j := i + 1
		switch {
		case isWordRune(runes[i]):
			for j < len(runes) && isWordRune(runes[j]) {
				j++
			}
		case unicode.IsSpace(runes[i]):
			for j < len(runes) && unicode.IsSpace(runes[j]) {
				j++
			}
		}
		out = append(out, string(runes[i:j]))
		i = j
	}
	return out
}

// markChanges sets the spans of every deleted row paired by position with an
// added row of its chunk, the pairing a side-by-side layout shows.
func markChanges(rows []diffRow) {
	var dels, adds []int
	flush := func() {
		for i := range min(len(dels), len(adds)) {
			d, a := &rows[dels[i]], &rows[adds[i]]
			d.spans, a.spans = changedSpans(d.text, a.text)
		}
		dels, adds = dels[:0], adds[:0]
	}
	for i, r := range rows {
		switch {
		case r.kind == rowDeleted:
			dels = append(dels, i)
		case r.kind == rowAdded:
			adds = append(adds, i)
		default:
			flush()
		}
	}
	flush()
}
