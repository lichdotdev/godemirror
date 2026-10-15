// Ported from @lezer/common 1.5.3 (src/tree.ts: TreeFragment,
// ChangedRange), MIT licensed, Copyright (C) 2018 by Marijn Haverbeke and
// others. See LICENSE.

package lezer

// ChangedRange is one edit between two versions of a document: [FromA,
// ToA) in the old one became [FromB, ToB) in the new one.
type ChangedRange struct {
	FromA, ToA, FromB, ToB int
}

// TreeFragment is a piece of an earlier tree an incremental parse may
// reuse: the unchanged range [From, To) of the new document, and the
// Offset to add to go from a new position to the tree's. OpenStart and
// OpenEnd say whether the fragment's ends are the ends of a change rather
// than of the parse, where nodes may not be safe to reuse.
type TreeFragment struct {
	From, To           int
	Tree               *Tree
	Offset             int
	OpenStart, OpenEnd bool
}

// noChangeAt stands for "no further change" in ApplyChanges, @lezer's 1e9.
const noChangeAt = 1_000_000_000

// defaultMinGap is applyChanges' default minGap: an unchanged stretch
// shorter than this is not worth a fragment.
const defaultMinGap = 128

// AddTree is the fragments of a fresh tree, with the fragments that reach
// past its end kept, as TreeFragment.addTree does. A partial tree's
// fragment is open at its end.
func AddTree(tree *Tree, fragments []TreeFragment, partial bool) []TreeFragment {
	result := []TreeFragment{{From: 0, To: tree.length, Tree: tree, OpenEnd: partial}}
	for _, f := range fragments {
		if f.To > tree.length {
			result = append(result, f)
		}
	}
	return result
}

// ApplyChanges drops or cuts the fragments the changes touch and moves the
// ones after them, as TreeFragment.applyChanges does with its default
// minGap. changes are in order and do not overlap.
func ApplyChanges(fragments []TreeFragment, changes []ChangedRange) []TreeFragment {
	if len(changes) == 0 {
		return fragments
	}
	var result []TreeFragment
	fI, next := 1, (*TreeFragment)(nil)
	if len(fragments) > 0 {
		next = &fragments[0]
	}
	for cI, pos, off := 0, 0, 0; ; cI++ {
		nextPos := noChangeAt
		if cI < len(changes) {
			nextPos = changes[cI].FromA
		}
		if nextPos-pos >= defaultMinGap {
			for next != nil && next.From < nextPos {
				if cut, ok := cutFragment(*next, pos, nextPos, off, cI > 0, cI < len(changes)); ok {
					result = append(result, cut)
				}
				if next.To > nextPos {
					break
				}
				next = nil
				if fI < len(fragments) {
					next = &fragments[fI]
					fI++
				}
			}
		}
		if cI == len(changes) {
			return result
		}
		pos, off = changes[cI].ToA, changes[cI].ToA-changes[cI].ToB
	}
}

// cutFragment is f limited to the unchanged stretch [pos, nextPos) of the
// old document and moved back by off, false when nothing is left.
func cutFragment(f TreeFragment, pos, nextPos, off int, openStart, openEnd bool) (TreeFragment, bool) {
	if pos < f.From && nextPos > f.To && off == 0 {
		return f, true
	}
	from, to := max(f.From, pos)-off, min(f.To, nextPos)-off
	if from >= to {
		return TreeFragment{}, false
	}
	return TreeFragment{From: from, To: to, Tree: f.Tree, Offset: f.Offset + off, OpenStart: openStart, OpenEnd: openEnd}, true
}
