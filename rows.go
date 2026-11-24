package godemirror

import "sort"

// tabSize is CodeMirror's default, which is what the web viewer renders with.
const tabSize = 4

// wrapLine returns where each visual row of l starts when it wraps at cols
// cells, breaking after a space when the row has one, as CSS pre-wrap does,
// and mid-word only when a word is longer than a row, never inside a wide
// character: one that cannot fit a row of one cell takes the row alone.
// cols <= 0 never wraps. Every row after the first is indented by hangOf, so
// it has that many cells less room.
func wrapLine(l []rune, cols int) []int {
	segments := []int{0}
	if cols <= 0 {
		return segments
	}
	room, continued := cols, cols-hangOf(l, cols)
	for start := 0; len(l)-start > room; {
		end := start + room
		for p := end; p > start; p-- {
			if l[p-1] == ' ' {
				end = p
				break
			}
		}
		if isTail(l[end]) {
			end--
		}
		if end == start {
			end += 2
		}
		if end == len(l) {
			break
		}
		segments = append(segments, end)
		start, room = end, continued
	}
	return segments
}

// hangingIndent is how many cells past the line's own indentation a wrapped
// row starts, so a continuation reads as part of the line above it.
const hangingIndent = 2

// hangOf is the cells a continuation row of l is indented by when l wraps at
// cols cells: its leading spaces and hangingIndent, but no more than half a
// row, so a deeply indented line keeps room for its text. It is virtual: the
// row's cells are still the line's own, from the break on.
func hangOf(l []rune, cols int) int {
	if cols <= 0 {
		return 0
	}
	lead := 0
	for lead < len(l) && l[lead] == ' ' {
		lead++
	}
	return min(lead+hangingIndent, cols/2)
}

// rowStarts is, per line, the first visual row it occupies, plus one final
// entry holding the total row count. A line takes as many rows as it wraps
// into, or minRows[i] when that is more: the rows a split view pads a line
// with to stay level with its other side.
func rowStarts(wraps [][]int, minRows []int) []int {
	starts := make([]int, len(wraps)+1)
	for i, w := range wraps {
		rows := len(w)
		if i < len(minRows) {
			rows = max(rows, minRows[i])
		}
		starts[i+1] = starts[i] + rows
	}
	return starts
}

// lineTops is, per line, the y its first row starts at, plus one final entry
// holding the document's height: rows of ch each, and under each line the
// extra[i] pixels of the blocks opened after it.
func lineTops(starts []int, ch int, extra []int) []int {
	tops := make([]int, len(starts))
	for i := range len(starts) - 1 {
		h := (starts[i+1] - starts[i]) * ch
		if i < len(extra) {
			h += extra[i]
		}
		tops[i+1] = tops[i] + h
	}
	return tops
}

// locate finds what sits at document height y: line, and the segment of it
// whose row is there, or -1 when y falls in the blocks under the line's rows.
// ok is false outside the document.
func locate(tops, starts []int, ch, y int) (line, segment int, ok bool) {
	if y < 0 || y >= tops[len(tops)-1] {
		return 0, 0, false
	}
	line = sort.Search(len(tops)-1, func(i int) bool { return tops[i+1] > y })
	segment = (y - tops[line]) / ch
	if segment >= starts[line+1]-starts[line] {
		segment = -1
	}
	return line, segment, true
}
