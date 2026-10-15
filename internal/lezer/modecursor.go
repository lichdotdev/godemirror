// Ported from @lezer/common 1.5.3 (src/tree.ts: IterMode, checkSide,
// TreeNode.nextChild and enter, TreeBuffer.findChild, TreeCursor), MIT
// licensed, Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.
// Only what incremental parsing walks trees with is here: enter always
// acts as if IgnoreOverlays were set, and EnterBracketed has no port.

package lezer

// iterMode is @lezer/common's IterMode.
type iterMode int

const (
	excludeBuffers   iterMode = 1
	includeAnonymous iterMode = 2
	ignoreMounts     iterMode = 4
)

// Sides a node may take relative to a position, as @lezer/common's Side.
const (
	sideBefore     = -2
	sideAtOrBefore = -1
	sideAround     = 0
	sideAtOrAfter  = 1
	sideAfter      = 2
	sideDontCare   = 4
)

func checkSide(side, pos, from, to int) bool {
	switch side {
	case sideBefore:
		return from < pos
	case sideAtOrBefore:
		return to >= pos && from < pos
	case sideAround:
		return from < pos && to > pos
	case sideAtOrAfter:
		return from <= pos && to > pos
	case sideAfter:
		return to > pos
	}
	return true
}

// findChildAt is TreeBuffer.findChild: the first (dir > 0) or last child
// between the indices that is on side of pos, -1 for none.
func (b *treeBuffer) findChildAt(startIndex, endIndex, dir, pos, side int) int {
	pick := -1
	for i := startIndex; i != endIndex; i = int(b.buffer[i+3]) {
		if checkSide(side, pos, int(b.buffer[i+1]), int(b.buffer[i+2])) {
			pick = i
			if dir > 0 {
				break
			}
		}
	}
	return pick
}

// nextChildMode is TreeNode.nextChild: the first child from index i on,
// going in direction dir, on side of pos, that a cursor in mode stops at.
func (n *treeNode) nextChildMode(i, dir, pos, side int, mode iterMode) (childRef, bool) {
	for parent := n; ; {
		children, positions := parent.tree.children, parent.tree.positions
		end := len(children)
		if dir < 0 {
			end = -1
		}
		for ; i != end; i += dir {
			next, start := children[i], positions[i]+parent.from
			if !checkSide(side, pos, start, start+next.length()) {
				continue
			}
			if ref, ok, stop := parent.childAtMode(i, dir, pos, side, mode, next, start); stop {
				return ref, ok
			}
		}
		if mode&includeAnonymous != 0 || !parent.tree.typ.Anonymous() {
			return childRef{}, false
		}
		if parent.index >= 0 {
			i = parent.index + dir
		} else if dir < 0 {
			i = -1
		} else {
			i = len(parent.parent.tree.children)
		}
		parent = parent.parent
		if parent == nil {
			return childRef{}, false
		}
	}
}

// childAtMode is nextChildMode's look at child i: stop is false when the
// scan goes on past it.
func (n *treeNode) childAtMode(i, dir, pos, side int, mode iterMode, next treeChild, start int) (ref childRef, ok, stop bool) {
	if next.buffer != nil {
		if mode&excludeBuffers != 0 {
			return childRef{}, false, false
		}
		index := next.buffer.findChildAt(0, len(next.buffer.buffer), dir, pos-start, side)
		if index < 0 {
			return childRef{}, false, false
		}
		return childRef{buffer: &bufferContext{parent: n, buffer: next.buffer, index: i, start: start}, index: index}, true, true
	}
	if mode&includeAnonymous == 0 && next.tree.typ.Anonymous() && !next.tree.hasChild() {
		return childRef{}, false, false
	}
	if m := next.tree.mounted; mode&ignoreMounts == 0 && m != nil && m.overlay == nil {
		return childRef{node: &treeNode{tree: m.tree, from: start, index: i, parent: n}}, true, true
	}
	inner := &treeNode{tree: next.tree, from: start, index: i, parent: n}
	if mode&includeAnonymous != 0 || !inner.tree.typ.Anonymous() {
		return childRef{node: inner}, true, true
	}
	first := 0
	if dir < 0 {
		first = len(next.tree.children) - 1
	}
	ref, ok = inner.nextChildMode(first, dir, pos, side, mode)
	return ref, ok, true
}

// modeCursor is TreeCursor in any mode but EnterBracketed.
type modeCursor struct {
	cursor
	mode iterMode
}

func newModeCursor(t *Tree, mode iterMode) *modeCursor {
	c := &modeCursor{mode: mode}
	c.yieldNode(&treeNode{tree: t})
	return c
}

func (c *modeCursor) enterChild(dir, pos, side int) bool {
	if c.buffer == nil {
		first := 0
		if dir < 0 {
			first = len(c.tree.tree.children) - 1
		}
		return c.yield(c.tree.nextChildMode(first, dir, pos, side, c.mode))
	}
	b := c.buffer.buffer
	index := b.findChildAt(c.index+4, int(b.buffer[c.index+3]), dir, pos-c.buffer.start, side)
	if index < 0 {
		return false
	}
	c.stack = append(c.stack, c.index)
	return c.yieldBuf(index)
}

func (c *modeCursor) childBefore(pos int) bool { return c.enterChild(-1, pos, sideBefore) }
func (c *modeCursor) childAfter(pos int) bool  { return c.enterChild(1, pos, sideAfter) }

// enter is TreeCursor.enter with mode, overlays never entered.
func (c *modeCursor) enter(pos, side int, mode iterMode) bool {
	if c.buffer == nil {
		return c.yield(c.tree.nextChildMode(0, 1, pos, side, mode))
	}
	if mode&excludeBuffers != 0 {
		return false
	}
	return c.enterChild(1, pos, side)
}

func (c *modeCursor) parent() bool {
	if c.buffer == nil {
		if c.mode&includeAnonymous != 0 {
			return c.yieldNode(c.tree.parent)
		}
		return c.yieldNode(c.tree.significantParent())
	}
	if n := len(c.stack); n > 0 {
		index := c.stack[n-1]
		c.stack = c.stack[:n-1]
		return c.yieldBuf(index)
	}
	parent := c.buffer.parent
	if c.mode&includeAnonymous == 0 {
		for parent.tree.typ.Anonymous() && parent.parent != nil {
			parent = parent.parent
		}
	}
	c.buffer = nil
	return c.yieldNode(parent)
}

func (c *modeCursor) sibling(dir int) bool {
	if c.buffer == nil {
		if c.tree.parent == nil || c.tree.index < 0 {
			return false
		}
		return c.yield(c.tree.parent.nextChildMode(c.tree.index+dir, dir, 0, sideDontCare, c.mode))
	}
	b, d := c.buffer.buffer, len(c.stack)-1
	if dir < 0 {
		parentStart := 0
		if d >= 0 {
			parentStart = c.stack[d] + 4
		}
		if c.index != parentStart {
			return c.yieldBuf(b.findChildAt(parentStart, c.index, -1, 0, sideDontCare))
		}
	} else {
		after, limit := int(b.buffer[c.index+3]), len(b.buffer)
		if d >= 0 {
			limit = int(b.buffer[c.stack[d]+3])
		}
		if after < limit {
			return c.yieldBuf(after)
		}
	}
	if d >= 0 {
		return false
	}
	return c.yield(c.buffer.parent.nextChildMode(c.buffer.index+dir, dir, 0, sideDontCare, c.mode))
}

// atLastNode reports whether nothing a cursor in this mode stops at comes
// after (dir > 0) or before the current node.
func (c *modeCursor) atLastNode(dir int) bool {
	var index int
	var parent *treeNode
	if c.buffer != nil {
		b := c.buffer.buffer.buffer
		if dir > 0 {
			if c.index < len(b) {
				return false
			}
		} else {
			for i := 0; i < c.index; i++ {
				if int(b[i+3]) < c.index {
					return false
				}
			}
		}
		index, parent = c.buffer.index, c.buffer.parent
	} else {
		index, parent = c.tree.index, c.tree.parent
	}
	for ; parent != nil; index, parent = parent.index, parent.parent {
		if index > -1 && c.hasStopBetween(parent, index, dir) {
			return false
		}
	}
	return true
}

// hasStopBetween reports whether a child of parent after (dir > 0) or
// before index is one the cursor would stop at.
func (c *modeCursor) hasStopBetween(parent *treeNode, index, dir int) bool {
	end := len(parent.tree.children)
	if dir < 0 {
		end = -1
	}
	for i := index + dir; i != end; i += dir {
		child := parent.tree.children[i]
		if c.mode&includeAnonymous != 0 || child.buffer != nil || !child.tree.typ.Anonymous() || child.tree.hasChild() {
			return true
		}
	}
	return false
}

// next is TreeCursor.next: the next node in pre-order, not entering the
// current one unless enter.
func (c *modeCursor) next(enter bool) bool {
	if enter && c.enterChild(1, 0, sideDontCare) {
		return true
	}
	for {
		if c.sibling(1) {
			return true
		}
		if c.atLastNode(1) || !c.parent() {
			return false
		}
	}
}

// moveTo moves to the innermost node covering pos, as TreeCursor.moveTo.
func (c *modeCursor) moveTo(pos, side int) {
	for c.from == c.to || (side < 1 && c.from >= pos) || (side >= 1 && c.from > pos) ||
		(side > -1 && c.to <= pos) || (side <= -1 && c.to < pos) {
		if !c.parent() {
			break
		}
	}
	for c.enterChild(1, pos, side) {
	}
}
