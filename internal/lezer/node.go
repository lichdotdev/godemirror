// Ported from @lezer/common 1.5.3 (src/tree.ts: TreeNode and BufferNode's
// navigation), MIT licensed, Copyright (C) 2018 by Marijn Haverbeke and
// others. See LICENSE.

package lezer

// Node is a node of a tree in place, as @lezer/common's SyntaxNode: a
// Tree, or a node inside a TreeBuffer. Its navigation skips anonymous
// nodes and enters mounted trees that replace their node.
type Node struct {
	tree *treeNode
	// Inside a buffer: the buffer, the node's index in it and its parent
	// inside the same buffer, nil at the buffer's top level.
	buffer *bufferContext
	index  int
	parent *Node
}

// nodeOf is the node a child lookup found, nil for none.
func nodeOf(ref childRef, ok bool) *Node {
	if !ok {
		return nil
	}
	if ref.node != nil {
		return &Node{tree: ref.node}
	}
	return &Node{buffer: ref.buffer, index: ref.index}
}

// Type is the node's type.
func (n *Node) Type() *NodeType {
	if n.buffer == nil {
		return n.tree.tree.typ
	}
	b := n.buffer.buffer
	return &b.types[b.buffer[n.index]]
}

// From is where the node starts, in UTF-16 code units.
func (n *Node) From() int {
	if n.buffer == nil {
		return n.tree.from
	}
	return n.buffer.start + int(n.buffer.buffer.buffer[n.index+1])
}

// To is where the node ends, in UTF-16 code units.
func (n *Node) To() int {
	if n.buffer == nil {
		return n.tree.from + n.tree.tree.length
	}
	return n.buffer.start + int(n.buffer.buffer.buffer[n.index+2])
}

// FirstChild is the node's first child, nil for none.
func (n *Node) FirstChild() *Node { return n.child(1) }

// LastChild is the node's last child, nil for none.
func (n *Node) LastChild() *Node { return n.child(-1) }

func (n *Node) child(dir int) *Node {
	if n.buffer == nil {
		first := 0
		if dir < 0 {
			first = len(n.tree.tree.children) - 1
		}
		return nodeOf(n.tree.nextChild(first, dir))
	}
	b := n.buffer.buffer
	index := b.findChild(n.index+4, int(b.buffer[n.index+3]), dir)
	if index < 0 {
		return nil
	}
	return &Node{buffer: n.buffer, index: index, parent: n}
}

// NextSibling is the node after this one in its parent, nil for none.
func (n *Node) NextSibling() *Node {
	if n.buffer == nil {
		if n.tree.parent == nil || n.tree.index < 0 {
			return nil
		}
		return nodeOf(n.tree.parent.nextChild(n.tree.index+1, 1))
	}
	b := n.buffer.buffer.buffer
	after, limit := int(b[n.index+3]), len(b)
	if n.parent != nil {
		limit = int(b[n.parent.index+3])
	}
	if after < limit {
		return &Node{buffer: n.buffer, index: after, parent: n.parent}
	}
	if n.parent != nil {
		return nil
	}
	return nodeOf(n.buffer.parent.nextChild(n.buffer.index+1, 1))
}

// Parent is the nearest ancestor that is not anonymous, nil for the root.
func (n *Node) Parent() *Node {
	if n.buffer == nil {
		if p := n.tree.significantParent(); p != nil {
			return &Node{tree: p}
		}
		return nil
	}
	if n.parent != nil {
		return n.parent
	}
	p := n.buffer.parent
	for p.tree.typ.Anonymous() && p.parent != nil {
		p = p.parent
	}
	return &Node{tree: p}
}
