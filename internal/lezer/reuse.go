// Ported from @lezer/lr 1.4.11 (src/parse.ts: cutAt, FragmentCursor,
// Parse.tryReuse and the reuse in advanceStack; src/stack.ts: useNode),
// MIT licensed, Copyright (C) 2018 by Marijn Haverbeke and others. See
// LICENSE.

package lezer

// noFragment is FragmentCursor's 1e9: no node to reuse ahead.
const noFragment = 1_000_000_000

// cutAt is the closest position to pos, going in direction side, where a
// node of tree ends (side < 0) or starts, past lookAheadMargin of pos and
// not in an error node: the safe edge of a fragment that ends or starts at
// a change.
func cutAt(tree *Tree, pos, side int) int {
	c := newModeCursor(tree, includeAnonymous)
	c.moveTo(pos, sideAround)
	for {
		var entered bool
		if side < 0 {
			entered = c.childBefore(pos)
		} else {
			entered = c.childAfter(pos)
		}
		if entered {
			continue
		}
		for {
			past := c.from > pos
			if side < 0 {
				past = c.to < pos
			}
			if past && !c.typ.Error {
				if side < 0 {
					return max(0, min(c.to-1, pos-lookAheadMargin))
				}
				return min(tree.length, max(c.from+1, pos+lookAheadMargin))
			}
			if c.sibling(side) {
				break
			}
			if !c.parent() {
				if side < 0 {
					return 0
				}
				return tree.length
			}
		}
	}
}

// fragmentCursor walks the trees of the fragments for nodes to reuse.
type fragmentCursor struct {
	fragments        []TreeFragment
	minRepeat        int
	i                int
	fragment         *TreeFragment
	safeFrom, safeTo int
	trees            []*Tree
	start, index     []int
	nextStart        int
	lastRepeat       *Tree
	lastRepeatPos    int
}

func newFragmentCursor(fragments []TreeFragment, minRepeat int) *fragmentCursor {
	fc := &fragmentCursor{fragments: fragments, minRepeat: minRepeat, safeFrom: -1, safeTo: -1, lastRepeatPos: -1}
	fc.nextFragment()
	return fc
}

func (fc *fragmentCursor) nextFragment() {
	if fc.i == len(fc.fragments) {
		fc.fragment = nil
		fc.nextStart = noFragment
		return
	}
	fr := &fc.fragments[fc.i]
	fc.fragment = fr
	fc.i++
	fc.safeFrom, fc.safeTo = fr.From, fr.To
	if fr.OpenStart {
		fc.safeFrom = cutAt(fr.Tree, fr.From+fr.Offset, 1) - fr.Offset
	}
	if fr.OpenEnd {
		fc.safeTo = cutAt(fr.Tree, fr.To+fr.Offset, -1) - fr.Offset
	}
	fc.trees, fc.start, fc.index = append(fc.trees[:0], fr.Tree), append(fc.start[:0], -fr.Offset), append(fc.index[:0], 0)
	fc.nextStart = fc.safeFrom
}

// nodeAt is a Tree of a fragment starting at pos that is safe to reuse,
// nil for none. pos never goes back between calls.
func (fc *fragmentCursor) nodeAt(pos int) *Tree {
	if pos < fc.nextStart {
		return nil
	}
	for fc.fragment != nil && fc.safeTo <= pos {
		fc.nextFragment()
	}
	if fc.fragment == nil {
		return nil
	}
	for {
		last := len(fc.trees) - 1
		if last < 0 {
			fc.nextFragment()
			return nil
		}
		top, index := fc.trees[last], fc.index[last]
		if index == len(top.children) {
			fc.pop()
			continue
		}
		next, start := top.children[index], fc.start[last]+top.positions[index]
		if start > pos {
			fc.nextStart = start
			return nil
		}
		if next.buffer != nil {
			fc.index[last]++
			fc.nextStart = start + next.buffer.length
			continue
		}
		if found, done := fc.look(next.tree, start, pos); done {
			return found
		}
		fc.index[last]++
		if start+next.tree.length >= max(fc.safeFrom, pos) {
			fc.trees, fc.start, fc.index = append(fc.trees, next.tree), append(fc.start, start), append(fc.index, 0)
		}
	}
}

func (fc *fragmentCursor) pop() {
	last := len(fc.trees) - 1
	fc.trees, fc.start, fc.index = fc.trees[:last], fc.start[:last], fc.index[:last]
}

// look checks the tree at start for nodeAt, noting it as the last repeat
// node seen. done is true when nodeAt has its answer.
func (fc *fragmentCursor) look(next *Tree, start, pos int) (found *Tree, done bool) {
	storeRepeat := start > fc.lastRepeatPos && next.typ.ID >= fc.minRepeat
	if start != pos && !storeRepeat {
		return nil, false
	}
	end := start + next.length
	safe := start >= fc.safeFrom && end <= fc.safeTo && (next.lookAhead == 0 || end+next.lookAhead < fc.fragment.To)
	if !safe {
		return nil, start == pos
	}
	if storeRepeat {
		fc.lastRepeat, fc.lastRepeatPos = next, start
	}
	if start == pos {
		return next, true
	}
	return nil, false
}

// tryReuse puts tree, or a descendant right at its start, on the stack
// when the stack can take it, and reports whether one was.
func (ps *parse) tryReuse(stack *Stack, start int, tree *Tree, repeatOnly bool) bool {
	p := ps.parser
	strictCx := stack.curContext != nil && !stack.curContext.tracker.NonStrict
	cxHash := 0
	if strictCx {
		cxHash = stack.curContext.hash
	}
	for tree != nil {
		match := -1
		if id := tree.typ.ID; id < len(p.types) && &p.types[id] == tree.typ {
			match = p.getGoto(stack.state, id, false)
		}
		if match > -1 && tree.length > 0 && (!strictCx || tree.contextHash == cxHash) {
			stack.useNode(tree, match, start)
			return true
		}
		if len(tree.children) == 0 || tree.positions[0] > 0 {
			break
		}
		inner := tree.children[0].tree
		if inner == nil || (repeatOnly && inner.typ.ID < p.minRepeatTerm) {
			break
		}
		tree = inner
	}
	return false
}

// reuseAt reuses a fragment's node at the stack's position, or the repeat
// node that starts before the skipped content in front of it, as the
// start of Parse.advanceStack does.
func (ps *parse) reuseAt(stack *Stack) bool {
	fc := ps.fragments
	if stack.reducePos < stack.pos && len(stack.buffer) > 0 && stack.buffer[len(stack.buffer)-2] <= stack.reducePos &&
		fc.lastRepeatPos == stack.reducePos && ps.tryReuse(stack, stack.reducePos, fc.lastRepeat, true) {
		return true
	}
	return ps.tryReuse(stack, stack.pos, fc.nodeAt(stack.pos), false)
}

// useNode shifts a node reused from an earlier tree.
func (s *Stack) useNode(value *Tree, next, start int) {
	reused := &s.p.reused
	index := len(*reused) - 1
	if index < 0 || (*reused)[index] != value {
		*reused = append(*reused, value)
		index++
	}
	end := start + value.length
	s.reducePos, s.pos = end, end
	s.pushState(next, start)
	s.buffer = append(s.buffer, index, start, end, recordReuse)
	if s.curContext != nil && s.curContext.tracker.Reuse != nil {
		s.updateContext(s.curContext.tracker.Reuse(s.curContext.context, value, s, s.p.stream.reset(s.pos-value.length, nil)))
	}
}
