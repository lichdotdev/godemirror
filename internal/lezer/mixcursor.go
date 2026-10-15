// Ported from @lezer/common 1.5.3 (src/tree.ts: TreeCursor in the
// IncludeAnonymous | IgnoreMounts mode MixedParse scans with, BufferNode's
// toTree, TreeBuffer.slice; src/mix.ts: sliceBuf, materialize), MIT
// licensed, Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

// noneType is NodeType.none, the anonymous type of the trees materialize
// wraps buffer slices in.
var noneType = NodeType{Props: map[string]string{}}

// scanCursor walks every node of a tree, anonymous ones included, without
// entering mounted trees.
type scanCursor struct {
	cursor
}

// nextChildAll is the first child from index i on, as nextChild finds it
// in the IncludeAnonymous | IgnoreMounts mode.
func (n *treeNode) nextChildAll(i int) (childRef, bool) {
	for ; i < len(n.tree.children); i++ {
		child, start := n.tree.children[i], n.tree.positions[i]+n.from
		if child.tree != nil {
			return childRef{node: &treeNode{tree: child.tree, from: start, index: i, parent: n}}, true
		}
		if index := child.buffer.findChild(0, len(child.buffer.buffer), 1); index >= 0 {
			return childRef{buffer: &bufferContext{parent: n, buffer: child.buffer, index: i, start: start}, index: index}, true
		}
	}
	return childRef{}, false
}

func (c *scanCursor) firstChild() bool {
	if c.buffer == nil {
		return c.yield(c.tree.nextChildAll(0))
	}
	return c.cursor.firstChild()
}

func (c *scanCursor) parent() bool {
	if c.buffer == nil {
		return c.yieldNode(c.tree.parent)
	}
	if n := len(c.stack); n > 0 {
		index := c.stack[n-1]
		c.stack = c.stack[:n-1]
		return c.yieldBuf(index)
	}
	parent := c.buffer.parent
	c.buffer = nil
	return c.yieldNode(parent)
}

func (c *scanCursor) nextSibling() bool {
	if c.buffer == nil {
		if c.tree.parent == nil || c.tree.index < 0 {
			return false
		}
		return c.yield(c.tree.parent.nextChildAll(c.tree.index + 1))
	}
	b, d := c.buffer.buffer.buffer, len(c.stack)-1
	after, limit := int(b[c.index+3]), len(b)
	if d >= 0 {
		limit = int(b[c.stack[d]+3])
	}
	if after < limit {
		return c.yieldBuf(after)
	}
	if d >= 0 {
		return false
	}
	return c.yield(c.buffer.parent.nextChildAll(c.buffer.index + 1))
}

// node is the cursor's node as a Node.
func (c *scanCursor) node() *Node {
	if c.buffer == nil {
		return &Node{tree: c.tree}
	}
	var parent *Node
	for _, index := range c.stack {
		parent = &Node{buffer: c.buffer, index: index, parent: parent}
	}
	return &Node{buffer: c.buffer, index: c.index, parent: parent}
}

// materialize turns the buffer node at the cursor, and the buffer nodes
// around it, into Trees, so a mounted tree can be stored on it, and moves
// the cursor onto its Tree. The buffer is the parse's own, so changing the
// tree it is in is safe.
func (c *scanCursor) materialize() {
	target := c.node()
	buf := c.buffer.buffer
	var stack []int
	for c.buffer != nil {
		stack = append(stack, c.index)
		c.parent()
	}
	base := c.tree.tree
	i := 0
	for base.children[i].buffer != buf {
		i++
	}
	m := &materializer{buf: buf, stack: stack, target: target, newStack: []int{i}}
	base.children[i] = treeChild{tree: m.split(0, len(buf.buffer), &noneType, 0, buf.length, len(stack)-1)}
	for _, index := range m.newStack {
		t := c.tree.tree
		c.yieldNode(&treeNode{tree: t.children[index].tree, from: t.positions[index] + c.from, index: index, parent: c.tree})
	}
}

type materializer struct {
	buf      *treeBuffer
	stack    []int
	target   *Node
	newStack []int
}

// split makes the buffer nodes in [startI, endI) a Tree of type typ: the
// ones before and after the node holding the target as buffers of their
// own, that node a Tree split the same way.
func (m *materializer) split(startI, endI int, typ *NodeType, innerOffset, length, stackPos int) *Tree {
	b := m.buf.buffer
	targetI := m.stack[stackPos]
	t := &Tree{typ: typ, length: length}
	m.slice(t, startI, targetI, innerOffset)
	from, to := int(b[targetI+1]), int(b[targetI+2])
	m.newStack = append(m.newStack, len(t.children))
	var child *Tree
	if stackPos > 0 {
		child = m.split(targetI+4, int(b[targetI+3]), &m.buf.types[b[targetI]], from, to-from, stackPos-1)
	} else {
		child = bufferNodeTree(m.target)
	}
	t.children = append(t.children, treeChild{tree: child})
	t.positions = append(t.positions, from-innerOffset)
	m.slice(t, int(b[targetI+3]), endI, innerOffset)
	return t
}

// slice adds the buffer nodes in [startI, endI) to t as one buffer.
func (m *materializer) slice(t *Tree, startI, endI, offset int) {
	if startI >= endI {
		return
	}
	from := int(m.buf.buffer[startI+1])
	t.children = append(t.children, treeChild{buffer: m.buf.slice(startI, endI, from)})
	t.positions = append(t.positions, from-offset)
}

// bufferNodeTree is BufferNode.toTree: the node as a Tree, its children
// one buffer.
func bufferNodeTree(n *Node) *Tree {
	b := n.buffer.buffer
	t := &Tree{typ: n.Type(), length: n.To() - n.From()}
	startI, endI := n.index+4, int(b.buffer[n.index+3])
	if endI > startI {
		t.children = []treeChild{{buffer: b.slice(startI, endI, int(b.buffer[n.index+1]))}}
		t.positions = []int{0}
	}
	return t
}

// slice is the nodes in [startI, endI) as a buffer of their own, positions
// made relative to from.
func (b *treeBuffer) slice(startI, endI, from int) *treeBuffer {
	out := make([]uint16, endI-startI)
	length := 0
	for i, j := startI, 0; i < endI; i, j = i+4, j+4 {
		out[j] = b.buffer[i]
		out[j+1] = b.buffer[i+1] - uint16(from)
		out[j+2] = b.buffer[i+2] - uint16(from)
		out[j+3] = b.buffer[i+3] - uint16(startI)
		length = max(length, int(out[j+2]))
	}
	return &treeBuffer{buffer: out, length: length, types: b.types}
}
