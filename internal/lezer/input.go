// Ported from @lezer/lr 1.4.11 (src/token.ts: InputStream, CachedToken),
// MIT licensed, Copyright (C) 2018 by Marijn Haverbeke and others. See
// LICENSE.

package lezer

import (
	"fmt"
	"unicode/utf16"
)

// Range is a span of the input, in UTF-16 code units.
type Range struct{ From, To int }

type cachedToken struct {
	start, value, end, extended, lookAhead, mask, context int
}

func newCachedToken() *cachedToken {
	return &cachedToken{start: -1, value: -1, end: -1, extended: -1}
}

// InputStream is how a tokenizer reads the input: as UTF-16 code units,
// the way JavaScript strings index, across the parse's ranges as if they
// were one text. The whole text is in memory, so @lezer/lr's chunking has
// no counterpart here.
type InputStream struct {
	input      []uint16
	ranges     []Range
	rng        Range
	rangeIndex int
	end        int
	nullToken  cachedToken
	token      *cachedToken
	// Next is the code unit at Pos, -1 at the end of the input.
	Next int
	// Pos is the stream's position in the input.
	Pos int
}

func newInputStream(input []uint16, ranges []Range) *InputStream {
	in := &InputStream{input: input, ranges: ranges, rng: ranges[0], Pos: ranges[0].From, end: ranges[len(ranges)-1].To}
	in.token = &in.nullToken
	in.readNext()
	return in
}

// resolveOffset is the position offset code units from Pos, skipping the
// gaps between ranges; ok is false when that leaves the input.
func (in *InputStream) resolveOffset(offset, assoc int) (int, bool) {
	rng, index := in.rng, in.rangeIndex
	pos := in.Pos + offset
	for pos < rng.From {
		if index == 0 {
			return 0, false
		}
		index--
		next := in.ranges[index]
		pos -= rng.From - next.To
		rng = next
	}
	for (assoc < 0 && pos > rng.To) || (assoc >= 0 && pos >= rng.To) {
		if index == len(in.ranges)-1 {
			return 0, false
		}
		index++
		next := in.ranges[index]
		pos += next.From - rng.To
		rng = next
	}
	return pos, true
}

func (in *InputStream) clipPos(pos int) int {
	if pos >= in.rng.From && pos < in.rng.To {
		return pos
	}
	for _, r := range in.ranges {
		if r.To > pos {
			return max(pos, r.From)
		}
	}
	return in.end
}

// Peek is the code unit offset units from Pos, -1 outside the input.
func (in *InputStream) Peek(offset int) int {
	pos, ok := in.resolveOffset(offset, 1)
	if !ok {
		return -1
	}
	if pos >= in.token.lookAhead {
		in.token.lookAhead = pos + 1
	}
	return int(in.input[pos])
}

// AcceptToken accepts a token of type term ending endOffset units from Pos.
// An end before the token's start is a tokenizer bug, and panics.
func (in *InputStream) AcceptToken(term, endOffset int) {
	end, ok := in.Pos, true
	if endOffset != 0 {
		end, ok = in.resolveOffset(endOffset, -1)
	}
	if !ok || end < in.token.start {
		panic(fmt.Sprintf("lezer: token end out of bounds (offset %d from %d)", endOffset, in.Pos))
	}
	in.token.value = term
	in.token.end = end
}

// AcceptTokenTo accepts a token of type term ending at endPos.
func (in *InputStream) AcceptTokenTo(term, endPos int) {
	in.token.value = term
	in.token.end = endPos
}

func (in *InputStream) readNext() int {
	if in.Pos >= in.rng.To {
		in.Next = -1
	} else {
		in.Next = int(in.input[in.Pos])
	}
	return in.Next
}

// Advance moves the stream n code units forward and returns the new Next.
func (in *InputStream) Advance(n int) int {
	for in.Pos+n >= in.rng.To {
		if in.rangeIndex == len(in.ranges)-1 {
			return in.setDone()
		}
		n -= in.rng.To - in.Pos
		in.rangeIndex++
		in.rng = in.ranges[in.rangeIndex]
		in.Pos = in.rng.From
	}
	in.Pos += n
	if in.Pos >= in.token.lookAhead {
		in.token.lookAhead = in.Pos + 1
	}
	return in.readNext()
}

func (in *InputStream) setDone() int {
	in.Pos = in.end
	in.rangeIndex = len(in.ranges) - 1
	in.rng = in.ranges[in.rangeIndex]
	in.Next = -1
	return -1
}

func (in *InputStream) reset(pos int, token *cachedToken) *InputStream {
	if token != nil {
		in.token = token
		token.start = pos
		token.lookAhead = pos + 1
		token.value, token.extended = -1, -1
	} else {
		in.token = &in.nullToken
	}
	if in.Pos == pos {
		return in
	}
	in.Pos = pos
	if pos == in.end {
		in.setDone()
		return in
	}
	for pos < in.rng.From {
		in.rangeIndex--
		in.rng = in.ranges[in.rangeIndex]
	}
	for pos >= in.rng.To {
		in.rangeIndex++
		in.rng = in.ranges[in.rangeIndex]
	}
	in.readNext()
	return in
}

// read is the text of [from, to), leaving out the gaps between ranges.
func (in *InputStream) read(from, to int) string {
	var units []uint16
	for _, r := range in.ranges {
		if r.From >= to {
			break
		}
		if r.To > from {
			units = append(units, in.input[max(r.From, from):min(r.To, to)]...)
		}
	}
	return string(utf16.Decode(units))
}
