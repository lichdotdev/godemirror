package godemirror

// maxDiffEdits caps the edit distance diffLines searches for between two
// stretches of lines. Myers' search costs O((N+M)·D) time, so two unrelated
// files of 100k lines would otherwise take minutes.
//
// ponytail: past the cap a stretch is shown as one chunk (all of it deleted,
// all of it inserted) rather than as its smallest diff; a caller who needs
// that for huge rewrites needs a heuristic like git's patience diff.
const maxDiffEdits = 10_000

type editOp uint8

const (
	opEqual editOp = iota
	opDelete
	opInsert
)

// edit is one line of an edit script: a line of a kept, deleted from a, or
// inserted from b.
type edit struct {
	op   editOp
	text string
}

// diffLines is a minimal edit script turning a into b, by Myers' linear-space
// algorithm: the middle snake of each stretch splits it in two, recursively.
// Within a run of changes the deletions come before the insertions. A stretch
// that needs more than maxEdits edits is deleted and inserted whole.
func diffLines(a, b []string, maxEdits int) []edit {
	ids := map[string]int{}
	intern := func(lines []string) []int {
		out := make([]int, len(lines))
		for i, l := range lines {
			id, ok := ids[l]
			if !ok {
				id = len(ids)
				ids[l] = id
			}
			out[i] = id
		}
		return out
	}
	m := myers{a: intern(a), b: intern(b), maxD: (maxEdits + 1) / 2}
	m.deleted, m.inserted = make([]bool, len(a)), make([]bool, len(b))
	m.compare(0, len(a), 0, len(b))
	return m.script(a, b)
}

// myers marks which lines of a are deleted and which of b inserted.
type myers struct {
	a, b              []int
	deleted, inserted []bool
	// maxD is how far each of the two searches of a middle snake may go.
	maxD int
}

func (m *myers) compare(aLo, aHi, bLo, bHi int) {
	for aLo < aHi && bLo < bHi && m.a[aLo] == m.b[bLo] {
		aLo++
		bLo++
	}
	for aLo < aHi && bLo < bHi && m.a[aHi-1] == m.b[bHi-1] {
		aHi--
		bHi--
	}
	if aLo == aHi || bLo == bHi {
		m.replace(aLo, aHi, bLo, bHi)
		return
	}
	x, y, ok := m.middleSnake(aLo, aHi, bLo, bHi)
	if !ok {
		m.replace(aLo, aHi, bLo, bHi)
		return
	}
	m.compare(aLo, x, bLo, y)
	m.compare(x, aHi, y, bHi)
}

func (m *myers) replace(aLo, aHi, bLo, bHi int) {
	for i := aLo; i < aHi; i++ {
		m.deleted[i] = true
	}
	for j := bLo; j < bHi; j++ {
		m.inserted[j] = true
	}
}

// middleSnake searches from both ends of the stretch at once until the two
// searches overlap, and returns a point on a shortest edit path strictly
// inside it. ok is false when the stretch needs more than 2·maxD edits.
func (m *myers) middleSnake(aLo, aHi, bLo, bHi int) (x, y int, ok bool) {
	a, b := m.a[aLo:aHi], m.b[bLo:bHi]
	n, mm := len(a), len(b)
	maxD := min((n+mm+1)/2, m.maxD)
	forward, backward := newSearch(maxD), newSearch(maxD)
	delta := n - mm
	// With an odd delta the forward search finds the overlap, with an even
	// one the backward search does.
	odd := delta%2 != 0
	ahead := func(x, y int) bool { return a[x] == b[y] }
	behind := func(x, y int) bool { return a[n-x-1] == b[mm-y-1] }
	for d := range maxD {
		found := forward.step(d, n, mm, ahead, func(k, fx, fy int) bool {
			bx, ok := backward.at(delta - k)
			if odd && ok && fx >= n-bx {
				x, y = fx, fy
				return true
			}
			return false
		})
		if !found {
			found = backward.step(d, n, mm, behind, func(k, bx, _ int) bool {
				fx, ok := forward.at(delta - k)
				if !odd && ok && fx >= n-bx {
					x, y = fx, fx-(delta-k)
					return true
				}
				return false
			})
		}
		if found {
			return aLo + x, bLo + y, true
		}
	}
	return 0, 0, false
}

// search is one direction of a middle-snake search: on each diagonal k, how
// far along x its furthest path of the current cost reaches, -1 for nowhere
// yet. kStart and kEnd trim the diagonals that ran off the stretch.
type search struct {
	v            []int
	offset       int
	kStart, kEnd int
}

func newSearch(maxD int) *search {
	s := &search{v: make([]int, 2*(maxD+1)), offset: maxD + 1}
	for i := range s.v {
		s.v[i] = -1
	}
	s.v[s.offset+1] = 0
	return s
}

// at is how far the search reaches on diagonal k, ok false for not at all.
func (s *search) at(k int) (x int, ok bool) {
	i := s.offset + k
	if i < 0 || i >= len(s.v) || s.v[i] == -1 {
		return 0, false
	}
	return s.v[i], true
}

// step extends every path of cost d-1 by one edit and then along the snake
// of equal lines that follows (match), and asks done of each furthest point
// still inside the stretch; it stops at the first done.
func (s *search) step(d, n, m int, match func(x, y int) bool, done func(k, x, y int) bool) bool {
	for k := -d + s.kStart; k <= d-s.kEnd; k += 2 {
		i := s.offset + k
		x := s.v[i-1] + 1
		if k == -d || (k != d && s.v[i-1] < s.v[i+1]) {
			x = s.v[i+1]
		}
		y := x - k
		for x < n && y < m && match(x, y) {
			x++
			y++
		}
		s.v[i] = x
		switch {
		case x > n:
			s.kEnd += 2
		case y > m:
			s.kStart += 2
		case done(k, x, y):
			return true
		}
	}
	return false
}

// script walks the marks into an edit script, deletions first in a run.
func (m *myers) script(a, b []string) []edit {
	out := make([]edit, 0, max(len(a), len(b)))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && m.deleted[i]:
			out = append(out, edit{opDelete, a[i]})
			i++
		case j < len(b) && m.inserted[j]:
			out = append(out, edit{opInsert, b[j]})
			j++
		default:
			out = append(out, edit{opEqual, a[i]})
			i++
			j++
		}
	}
	return out
}
