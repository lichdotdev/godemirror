package godemirror

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ChangeSet and the functions behind it are a port of @codemirror/state
// 6.7.6's change.ts. A set is a list of sections, each a pair of integers:
// the section's length in the document before the change, and -1 for a
// section left as it was, or the length of what replaces it. An insertion is
// (0, n), a deletion (n, 0), a replacement two positive numbers. Lengths are
// in bytes of the document with its lines joined by "\n".

// MapMode is how ChangeSet.MapPosTrack reports a position whose
// surroundings were deleted, as CodeMirror's MapMode does.
type MapMode int

const (
	// MapTrackDel reports a position that a deletion spans: the characters
	// on both of its sides are gone.
	MapTrackDel MapMode = iota + 1
	// MapTrackBefore reports a position whose character before it is gone.
	MapTrackBefore
	// MapTrackAfter reports a position whose character after it is gone.
	MapTrackAfter
)

// ChangeSpec replaces the bytes [From, To) of a document with Insert. An
// insertion has To == From, a deletion an empty Insert. Line breaks in
// Insert (LF, CRLF or CR, or only the editor's LineSeparator when it has
// one) become the document's line breaks.
type ChangeSpec struct {
	From, To int
	Insert   string
}

// ChangeSet is a group of changes to a document of one length, as
// CodeMirror's ChangeSet: an Update carries the one that took the document
// before it to the one after.
type ChangeSet struct {
	sections []int
	inserted []string
}

// Len is the length of the document before the changes.
func (c ChangeSet) Len() int {
	n := 0
	for i := 0; i < len(c.sections); i += 2 {
		n += c.sections[i]
	}
	return n
}

// NewLen is the length of the document after the changes.
func (c ChangeSet) NewLen() int {
	n := 0
	for i := 0; i < len(c.sections); i += 2 {
		if ins := c.sections[i+1]; ins < 0 {
			n += c.sections[i]
		} else {
			n += ins
		}
	}
	return n
}

// Empty reports whether the set changes nothing.
func (c ChangeSet) Empty() bool {
	return len(c.sections) == 0 || len(c.sections) == 2 && c.sections[1] < 0
}

// IterGaps calls f for each stretch the changes leave alone: where it starts
// in the old document (posA) and in the new one (posB), and its length.
func (c ChangeSet) IterGaps(f func(posA, posB, length int)) {
	posA, posB := 0, 0
	for i := 0; i < len(c.sections); i += 2 {
		n, ins := c.sections[i], c.sections[i+1]
		if ins < 0 {
			f(posA, posB, n)
			posB += n
		} else {
			posB += ins
		}
		posA += n
	}
}

// IterChanges calls f for each changed range, in document order: [fromA, toA)
// in the old document is replaced by [fromB, toB) in the new one, which holds
// inserted. Adjacent changes are reported as one.
func (c ChangeSet) IterChanges(f func(fromA, toA, fromB, toB int, inserted string)) {
	posA, posB := 0, 0
	for i := 0; i < len(c.sections); {
		n, ins := c.sections[i], c.sections[i+1]
		i += 2
		if ins < 0 {
			posA += n
			posB += n
			continue
		}
		endA, endB := posA, posB
		var text strings.Builder
		for {
			endA += n
			endB += ins
			if ins > 0 {
				text.WriteString(c.insertedAt(i - 2))
			}
			if i == len(c.sections) || c.sections[i+1] < 0 {
				break
			}
			n, ins = c.sections[i], c.sections[i+1]
			i += 2
		}
		f(posA, endA, posB, endB, text.String())
		posA, posB = endA, endB
	}
}

// insertedAt is the text of the section starting at sections[i].
func (c ChangeSet) insertedAt(i int) string {
	if i>>1 < len(c.inserted) {
		return c.inserted[i>>1]
	}
	return ""
}

// MapPos is where pos, a position in the document before the changes, ends
// up after them. assoc decides a position that text was inserted at or that
// a replacement spans: negative keeps it with the character before it (before
// the insertion), zero or positive with the character after. pos must be in
// [0, Len()].
func (c ChangeSet) MapPos(pos, assoc int) int {
	p, _ := c.mapPos(pos, assoc, 0)
	return p
}

// MapPosTrack is MapPos, but ok is false when the changes deleted what mode
// tracks around pos.
func (c ChangeSet) MapPosTrack(pos, assoc int, mode MapMode) (mapped int, ok bool) {
	return c.mapPos(pos, assoc, mode)
}

func (c ChangeSet) mapPos(pos, assoc int, mode MapMode) (int, bool) {
	posA, posB := 0, 0
	for i := 0; i < len(c.sections); i += 2 {
		n, ins := c.sections[i], c.sections[i+1]
		endA := posA + n
		if ins < 0 {
			if endA > pos {
				return posB + (pos - posA), true
			}
			posB += n
		} else {
			if mode != 0 && endA >= pos &&
				(mode == MapTrackDel && posA < pos && endA > pos ||
					mode == MapTrackBefore && posA < pos ||
					mode == MapTrackAfter && endA > pos) {
				return 0, false
			}
			if endA > pos || endA == pos && assoc < 0 && n == 0 {
				if pos == posA || assoc < 0 {
					return posB, true
				}
				return posB + ins, true
			}
			posB += ins
		}
		posA = endA
	}
	if pos > posA {
		panic(fmt.Sprintf("godemirror: position %d is out of range for a change set of length %d", pos, posA))
	}
	return posB, true
}

// String lists the sections as CodeMirror's ChangeDesc.toString does: "5" for
// five bytes kept, "2:3" for two replaced by three.
func (c ChangeSet) String() string {
	var b strings.Builder
	for i := 0; i < len(c.sections); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.Itoa(c.sections[i]))
		if ins := c.sections[i+1]; ins >= 0 {
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(ins))
		}
	}
	return b.String()
}

// apply is doc with the changes made. doc must be Len() bytes long.
func (c ChangeSet) apply(doc string) string {
	var b strings.Builder
	b.Grow(c.NewLen())
	pos := 0
	for i := 0; i < len(c.sections); i += 2 {
		n, ins := c.sections[i], c.sections[i+1]
		if ins < 0 {
			b.WriteString(doc[pos : pos+n])
		} else {
			b.WriteString(c.insertedAt(i))
		}
		pos += n
	}
	return b.String()
}

// invert is the set that takes the document these changes produce back to
// doc, the document before them.
func (c ChangeSet) invert(doc string) ChangeSet {
	sections := make([]int, len(c.sections))
	copy(sections, c.sections)
	var inserted []string
	pos := 0
	for i := 0; i < len(sections); i += 2 {
		n, ins := sections[i], sections[i+1]
		if ins >= 0 {
			sections[i], sections[i+1] = ins, n
			for len(inserted) < i>>1 {
				inserted = append(inserted, "")
			}
			inserted = append(inserted, doc[pos:pos+n])
		}
		pos += n
	}
	return ChangeSet{sections, inserted}
}

// desc is the changes without the text they insert, CodeMirror's
// ChangeDesc: enough to map positions and other changes through.
func (c ChangeSet) desc() ChangeSet { return ChangeSet{sections: c.sections} }

// invertedDesc is the desc of the changes that undo these.
func (c ChangeSet) invertedDesc() ChangeSet {
	sections := make([]int, len(c.sections))
	for i := 0; i < len(sections); i += 2 {
		n, ins := c.sections[i], c.sections[i+1]
		if ins < 0 {
			sections[i], sections[i+1] = n, ins
		} else {
			sections[i], sections[i+1] = ins, n
		}
	}
	return ChangeSet{sections: sections}
}

// compose is these changes followed by other, which must start from the
// document these produce.
func (c ChangeSet) compose(other ChangeSet) ChangeSet {
	switch {
	case c.Empty():
		return other
	case other.Empty():
		return c
	}
	return composeSets(c, other)
}

// mapOnto is these changes rebased to apply after other, which starts from
// the same document; at the same place, other's insertions come first.
func (c ChangeSet) mapOnto(other ChangeSet) ChangeSet {
	if other.Empty() {
		return c
	}
	return mapSet(c, other, false)
}

// mapOntoBefore is mapOnto, but at the same place these changes' insertions
// come first.
func (c ChangeSet) mapOntoBefore(other ChangeSet) ChangeSet {
	if other.Empty() {
		return c
	}
	return mapSet(c, other, true)
}

func emptyChangeSet(length int) ChangeSet {
	if length == 0 {
		return ChangeSet{}
	}
	return ChangeSet{sections: []int{length, -1}}
}

var errChangeLength = errors.New("godemirror: change sets of mismatched lengths")

// changeSetOf builds the set of specs for a document of length bytes, as
// ChangeSet.of does: specs out of order are composed in the order given, each
// mapped over the ones before it. start, when not nil, comes before specs, as
// a ChangeSet at the head of a CodeMirror ChangeSpec array.
func changeSetOf(specs []ChangeSpec, length int, lineSep string, start *ChangeSet) (ChangeSet, error) {
	var sections []int
	var inserted []string
	pos := 0
	var total *ChangeSet
	if start != nil {
		if start.Len() != length {
			return ChangeSet{}, errChangeLength
		}
		total = start
	}
	flush := func(force bool) {
		if !force && len(sections) == 0 {
			return
		}
		if pos < length {
			sections = addSection(sections, length-pos, -1, false)
		}
		set := ChangeSet{sections, inserted}
		if total != nil {
			set = total.compose(set.mapOnto(*total))
		}
		total = &set
		sections, inserted, pos = nil, nil, 0
	}
	for _, s := range specs {
		if s.From > s.To || s.From < 0 || s.To > length {
			return ChangeSet{}, fmt.Errorf("godemirror: invalid change range %d to %d (in a document of length %d)", s.From, s.To, length)
		}
		text := normalizeBreaks(s.Insert, lineSep)
		if s.From == s.To && text == "" {
			continue
		}
		if s.From < pos {
			flush(false)
		}
		if s.From > pos {
			sections = addSection(sections, s.From-pos, -1, false)
		}
		sections = addSection(sections, s.To-s.From, len(text), false)
		inserted = addInsert(inserted, sections, text)
		pos = s.To
	}
	flush(total == nil)
	return *total, nil
}

// normalizeBreaks turns text's line breaks into the "\n" the document joins
// its lines with: LF, CRLF and CR, or only lineSep when there is one.
func normalizeBreaks(text, lineSep string) string {
	if lineSep != "" {
		return strings.ReplaceAll(text, lineSep, "\n")
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

func addSection(sections []int, n, ins int, forceJoin bool) []int {
	if n == 0 && ins <= 0 {
		return sections
	}
	last := len(sections) - 2
	switch {
	case last >= 0 && ins <= 0 && ins == sections[last+1]:
		sections[last] += n
	case last >= 0 && n == 0 && sections[last] == 0:
		sections[last+1] += ins
	case forceJoin:
		sections[last] += n
		sections[last+1] += ins
	default:
		sections = append(sections, n, ins)
	}
	return sections
}

func addInsert(values []string, sections []int, value string) []string {
	if value == "" {
		return values
	}
	index := (len(sections) - 2) >> 1
	if index < len(values) {
		values[len(values)-1] += value
		return values
	}
	for len(values) < index {
		values = append(values, "")
	}
	return append(values, value)
}
