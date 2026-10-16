// Ported from @lezer/common 1.5.3 (src/tree.ts: TreeNode, BufferContext,
// TreeCursor in its default mode), MIT licensed, Copyright (C) 2018 by
// Marijn Haverbeke and others. See LICENSE.

package lezer

import "unicode/utf16"

// treeNode is a Tree in place: where it starts, its index in its parent's
// children, and that parent, anonymous ones included.
type treeNode struct {
	tree   *Tree
	from   int
	index  int
	parent *treeNode
}

// bufferContext is a TreeBuffer in place, as the child index of parent.
type bufferContext struct {
	parent *treeNode
	buffer *treeBuffer
	index  int
	start  int
}

// childRef is what nextChild finds: a tree node, or a node of a buffer.
type childRef struct {
	node   *treeNode
	buffer *bufferContext
	index  int
}

// nextChild is the first child from index i on, going in direction dir,
// that a default-mode cursor stops at: anonymous trees are entered, and
// a child with a mounted tree that has no overlay is that tree.
func (n *treeNode) nextChild(i, dir int) (childRef, bool) {
	for parent := n; ; {
		children, positions := parent.tree.children, parent.tree.positions
		end := len(children)
		if dir < 0 {
			end = -1
		}
		for ; i != end; i += dir {
			if ref, ok := parent.childAt(i, dir, children[i], positions[i]+parent.from); ok {
				return ref, true
			}
		}
		if !parent.tree.typ.Anonymous() || parent.parent == nil {
			return childRef{}, false
		}
		i = parent.index + dir
		parent = parent.parent
	}
}

func (n *treeNode) childAt(i, dir int, next treeChild, start int) (childRef, bool) {
	if next.buffer != nil {
		index := next.buffer.findChild(0, len(next.buffer.buffer), dir)
		if index < 0 {
			return childRef{}, false
		}
		return childRef{buffer: &bufferContext{parent: n, buffer: next.buffer, index: i, start: start}, index: index}, true
	}
	if next.tree.typ.Anonymous() && !next.tree.hasChild() {
		return childRef{}, false
	}
	if m := next.tree.mounted; m != nil && m.overlay == nil {
		return childRef{node: &treeNode{tree: m.tree, from: start, index: i, parent: n}}, true
	}
	inner := &treeNode{tree: next.tree, from: start, index: i, parent: n}
	if !inner.tree.typ.Anonymous() {
		return childRef{node: inner}, true
	}
	first := 0
	if dir < 0 {
		first = len(next.tree.children) - 1
	}
	return inner.nextChild(first, dir)
}

// significantParent is the nearest ancestor that is not anonymous, or the
// root.
func (n *treeNode) significantParent() *treeNode {
	if n.parent == nil {
		return nil
	}
	p := n.parent
	for p.tree.typ.Anonymous() && p.parent != nil {
		p = p.parent
	}
	return p
}

// matchNodeContext reports whether context[:i+1], nearest last, names node
// and its ancestors, "" matching any name.
func matchNodeContext(node *treeNode, context []string, i int) bool {
	for p := node; i >= 0; p = p.significantParent() {
		if p == nil {
			return false
		}
		if p.tree.typ.Anonymous() {
			continue
		}
		if context[i] != "" && context[i] != p.tree.typ.Name {
			return false
		}
		i--
	}
	return true
}

// cursor walks a tree as @lezer/common's TreeCursor does in its default
// mode. Inside a buffer, tree is not updated: it stays the tree node the
// cursor last stood on, which TreeCursor.matchContext then reads as the
// buffer's parent. That is a quirk of @lezer/common, kept because
// CodeMirror highlights with it.
type cursor struct {
	tree     *treeNode
	buffer   *bufferContext
	stack    []int
	index    int
	typ      *NodeType
	from, to int
}

func newCursor(t *Tree) *cursor {
	c := &cursor{}
	c.yieldNode(&treeNode{tree: t})
	return c
}

func (c *cursor) yieldNode(n *treeNode) bool {
	if n == nil {
		return false
	}
	c.tree, c.typ, c.from, c.to = n, n.tree.typ, n.from, n.from+n.tree.length
	return true
}

func (c *cursor) yieldBuf(index int) bool {
	c.index = index
	b := c.buffer.buffer.buffer
	c.typ = &c.buffer.buffer.types[b[index]]
	c.from = c.buffer.start + int(b[index+1])
	c.to = c.buffer.start + int(b[index+2])
	return true
}

func (c *cursor) yield(ref childRef, ok bool) bool {
	if !ok {
		return false
	}
	if ref.node != nil {
		c.buffer = nil
		return c.yieldNode(ref.node)
	}
	c.buffer = ref.buffer
	return c.yieldBuf(ref.index)
}

func (c *cursor) firstChild() bool {
	if c.buffer == nil {
		return c.yield(c.tree.nextChild(0, 1))
	}
	b := c.buffer.buffer
	index := b.findChild(c.index+4, int(b.buffer[c.index+3]), 1)
	if index < 0 {
		return false
	}
	c.stack = append(c.stack, c.index)
	return c.yieldBuf(index)
}

func (c *cursor) parent() bool {
	if c.buffer == nil {
		return c.yieldNode(c.tree.significantParent())
	}
	if n := len(c.stack); n > 0 {
		index := c.stack[n-1]
		c.stack = c.stack[:n-1]
		return c.yieldBuf(index)
	}
	parent := c.buffer.parent
	for parent.tree.typ.Anonymous() && parent.parent != nil {
		parent = parent.parent
	}
	c.buffer = nil
	return c.yieldNode(parent)
}

func (c *cursor) nextSibling() bool {
	if c.buffer == nil {
		if c.tree.parent == nil || c.tree.index < 0 {
			return false
		}
		return c.yield(c.tree.parent.nextChild(c.tree.index+1, 1))
	}
	b, d := c.buffer.buffer.buffer, len(c.stack)-1
	after := int(b[c.index+3])
	limit := len(b)
	if d >= 0 {
		limit = int(b[c.stack[d]+3])
	}
	if after < limit {
		return c.yieldBuf(after)
	}
	if d >= 0 {
		return false
	}
	return c.yield(c.buffer.parent.nextChild(c.buffer.index+1, 1))
}

// matchContext reports whether the node's ancestors, nearest last, are
// named as context lists them, "" matching any name.
func (c *cursor) matchContext(context []string) bool {
	if c.buffer == nil {
		return matchNodeContext(c.tree.significantParent(), context, len(context)-1)
	}
	b := c.buffer.buffer
	for i, d := len(context)-1, len(c.stack)-1; i >= 0; d-- {
		if d < 0 {
			return matchNodeContext(c.tree, context, i)
		}
		t := &b.types[b.buffer[c.stack[d]]]
		if t.Anonymous() {
			continue
		}
		if context[i] != "" && context[i] != t.Name {
			return false
		}
		i--
	}
	return true
}

// styleRule is the first of the node's styleTags rules whose context
// matches, as @lezer/highlight's getStyleTags picks it, and whose Names, if
// it has some, hold the node's text.
func (c *cursor) styleRule(text []uint16) *Rule {
	rules := c.typ.Rules
	for i := range rules {
		if rules[i].Context != nil && !c.matchContext(rules[i].Context) {
			continue
		}
		if rules[i].Names == nil || rules[i].Names[string(utf16.Decode(text[c.from:c.to]))] {
			return &rules[i]
		}
	}
	return nil
}
