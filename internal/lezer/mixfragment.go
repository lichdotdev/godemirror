// Ported from @lezer/common 1.5.3 (src/mix.ts: StructureCursor,
// FragmentCursor, findCoverChanges, enterFragments), MIT licensed,
// Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

// structureCursor walks the Trees of an earlier tree, buffers and mounts
// left out, in step with a scan of the new one.
type structureCursor struct {
	cursor *modeCursor
	offset int
	done   bool
}

func newStructureCursor(root *Tree, offset int) *structureCursor {
	return &structureCursor{cursor: newModeCursor(root, includeAnonymous|ignoreMounts|excludeBuffers), offset: offset}
}

// moveTo moves to the first node, in pre-order, that starts at or after
// pos.
func (sc *structureCursor) moveTo(pos int) {
	c, p := sc.cursor, pos-sc.offset
	for !sc.done && c.from < p {
		switch {
		case c.to >= p && c.enter(p, sideAtOrAfter, includeAnonymous|excludeBuffers):
		case c.to <= p:
			if !c.next(false) {
				sc.done = true
			}
		default:
			return
		}
	}
}

// hasNode reports whether the scan's node is a Tree of the earlier tree,
// or the first child at its start of one, which a parse reused whole.
func (sc *structureCursor) hasNode(from int, tree *Tree) bool {
	sc.moveTo(from)
	if sc.done || sc.cursor.from+sc.offset != from || sc.cursor.buffer != nil {
		return false
	}
	for t := sc.cursor.tree.tree; ; {
		if t == tree {
			return true
		}
		if len(t.children) == 0 || t.positions[0] != 0 || t.children[0].tree == nil {
			return false
		}
		t = t.children[0].tree
	}
}

// mixFragmentCursor finds, for a scan of a new tree, the nodes taken
// whole from the earlier trees of the fragments, and the trees mounted on
// them.
type mixFragmentCursor struct {
	fragments []TreeFragment
	fragI     int
	cur       *TreeFragment
	curTo     int
	inner     *structureCursor
}

func newMixFragmentCursor(fragments []TreeFragment) *mixFragmentCursor {
	fc := &mixFragmentCursor{fragments: fragments, fragI: -1}
	fc.nextFrag()
	return fc
}

func (fc *mixFragmentCursor) nextFrag() {
	fc.fragI++
	if fc.fragI >= len(fc.fragments) {
		fc.cur, fc.inner = nil, nil
		return
	}
	fc.cur = &fc.fragments[fc.fragI]
	fc.curTo = fc.cur.To
	fc.inner = newStructureCursor(fc.cur.Tree, -fc.cur.Offset)
}

// hasNode reports whether the scan's node, a Tree unless tree is nil, was
// reused from a fragment, its nested parses with it.
func (fc *mixFragmentCursor) hasNode(from, to int, tree *Tree) bool {
	for fc.cur != nil && from >= fc.curTo {
		fc.nextFrag()
	}
	return fc.cur != nil && fc.cur.From <= from && fc.curTo >= to && fc.inner.hasNode(from, tree)
}

// oldMount is a tree parser mounted in an earlier tree, around the start
// of a node the new tree nests again, and a fragment of that tree.
type oldMount struct {
	frag  TreeFragment
	pos   int
	mount *mountedTree
}

// findMounts is the trees parser mounted on pos's ancestors in the
// current fragment's tree.
func (fc *mixFragmentCursor) findMounts(pos int, parser *Parser) []oldMount {
	if fc.inner == nil {
		return nil
	}
	var result []oldMount
	c := fc.inner.cursor
	c.moveTo(pos, sideAtOrAfter)
	for n := c.tree; n != nil; n = n.significantParent() {
		mount := n.tree.mounted
		if mount == nil || mount.parser != parser {
			continue
		}
		for i := fc.fragI; i < len(fc.fragments); i++ {
			frag := fc.fragments[i]
			if frag.From >= n.from+n.tree.length {
				break
			}
			if frag.Tree == fc.cur.Tree {
				result = append(result, oldMount{frag: frag, pos: n.from - frag.Offset, mount: mount})
			}
		}
	}
	return result
}

// findCoverChanges is the parts of [from, to) inside exactly one of the
// range lists a and b.
func findCoverChanges(a, b []Range, from, to int) []Range {
	iA, iB, inA, inB, pos := 0, 0, false, false, -noFragment
	var result []Range
	edge := func(rs []Range, i int, in bool) int {
		switch {
		case i == len(rs):
			return noFragment
		case in:
			return rs[i].To
		}
		return rs[i].From
	}
	for {
		nextA, nextB := edge(a, iA, inA), edge(b, iB, inB)
		if inA != inB {
			if start, end := max(pos, from), min(nextA, nextB, to); start < end {
				result = append(result, Range{start, end})
			}
		}
		pos = min(nextA, nextB)
		if pos == noFragment {
			return result
		}
		if nextA == pos {
			if inA {
				iA++
			}
			inA = !inA
		}
		if nextB == pos {
			if inB {
				iB++
			}
			inB = !inB
		}
	}
}

// enterFragments is the fragments of the trees mounted in old that the
// inner parse of ranges may reuse.
func enterFragments(mounts []oldMount, ranges []Range) []TreeFragment {
	var result []TreeFragment
	for _, m := range mounts {
		startPos := m.pos
		if m.mount.overlay != nil {
			startPos += m.mount.overlay[0].From
		}
		endPos := startPos + m.mount.tree.length
		from, to := max(m.frag.From, startPos), min(m.frag.To, endPos)
		if m.mount.overlay == nil {
			result = append(result, TreeFragment{From: from, To: to, Tree: m.mount.tree, Offset: -startPos,
				OpenStart: m.frag.From >= startPos || m.frag.OpenStart, OpenEnd: m.frag.To <= endPos || m.frag.OpenEnd})
			continue
		}
		overlay := make([]Range, len(m.mount.overlay))
		for i, r := range m.mount.overlay {
			overlay[i] = Range{r.From + m.pos, r.To + m.pos}
		}
		changes := findCoverChanges(ranges, overlay, from, to)
		for i, pos := 0, from; ; i++ {
			last := i == len(changes)
			end := to
			if !last {
				end = changes[i].From
			}
			if end > pos {
				result = append(result, TreeFragment{From: pos, To: end, Tree: m.mount.tree, Offset: -startPos,
					OpenStart: m.frag.From >= pos || m.frag.OpenStart, OpenEnd: m.frag.To <= end || m.frag.OpenEnd})
			}
			if last {
				break
			}
			pos = changes[i].To
		}
	}
	return result
}
