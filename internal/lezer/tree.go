// Ported from @lezer/common 1.5.3 (src/tree.ts: Tree, TreeBuffer,
// Tree.build, balanceRange), MIT licensed, Copyright (C) 2018 by Marijn
// Haverbeke and others. See LICENSE.

package lezer

import (
	"regexp"
	"strconv"
	"strings"
)

// Tree is a syntax tree node with its children, as @lezer/common builds it:
// small subtrees are packed into TreeBuffers, and long repetitions are
// balanced under anonymous nodes. The layout is kept exactly, because a
// tree cursor's view of a node's context depends on it (see cursor.go).
// Positions are UTF-16 code units, relative to the tree's start.
type Tree struct {
	typ       *NodeType
	children  []treeChild
	positions []int
	length    int
	// mounted is a tree another parser made of this node's text, as
	// mountNested stores it: NodeProp.mounted.
	mounted *mountedTree
}

// treeChild is a Tree or a TreeBuffer.
type treeChild struct {
	tree   *Tree
	buffer *treeBuffer
}

func (c treeChild) length() int {
	if c.tree != nil {
		return c.tree.length
	}
	return c.buffer.length
}

// treeBuffer holds small subtrees as (type, from, to, end index) quads in
// pre-order, positions relative to the buffer's start.
type treeBuffer struct {
	buffer []uint16
	length int
	types  []NodeType
}

// defaultBufferLength is @lezer/common's DefaultBufferLength, the most
// text a TreeBuffer covers.
const defaultBufferLength = 1024

// Length is the length of the text the tree covers.
func (t *Tree) Length() int { return t.length }

// hasChild reports whether a cursor would stop anywhere inside t.
func (t *Tree) hasChild() bool {
	for _, c := range t.children {
		if c.buffer != nil || !c.tree.typ.Anonymous() || c.tree.hasChild() {
			return true
		}
	}
	return false
}

// String prints the tree as @lezer/common's Tree.toString does, as in
// `Program(ClassDeclaration(class,Definition,ClassBody("{","}")))`.
func (t *Tree) String() string {
	if t.mounted != nil && t.mounted.overlay == nil {
		return t.mounted.tree.String()
	}
	var parts []string
	for _, c := range t.children {
		s := ""
		if c.tree != nil {
			s = c.tree.String()
		} else {
			s = c.buffer.String()
		}
		if s != "" {
			parts = append(parts, s)
		}
	}
	return nodeString(t.typ, parts)
}

// String prints the buffer's top-level nodes, comma-separated.
func (b *treeBuffer) String() string {
	var parts []string
	for i := 0; i < len(b.buffer); i = int(b.buffer[i+3]) {
		parts = append(parts, b.childString(i))
	}
	return strings.Join(parts, ",")
}

func (b *treeBuffer) childString(index int) string {
	end := int(b.buffer[index+3])
	var parts []string
	for i := index + 4; i < end; i = int(b.buffer[i+3]) {
		parts = append(parts, b.childString(i))
	}
	return nodeString(&b.types[b.buffer[index]], parts)
}

// nodeString is a node named for its type, its children's strings in
// parentheses; an anonymous node is its children alone.
func nodeString(t *NodeType, children []string) string {
	inner := strings.Join(children, ",")
	if t.Name == "" {
		return inner
	}
	name := t.Name
	if nonWord.MatchString(name) && !t.Error {
		name = strconv.Quote(name)
	}
	if inner == "" {
		return name
	}
	return name + "(" + inner + ")"
}

// nonWord matches a name String quotes, as JavaScript's /\W/ does.
var nonWord = regexp.MustCompile(`[^A-Za-z0-9_]`)

// findChild is the index of the first (dir > 0) or last child of the node
// whose children run from startIndex to endIndex, -1 for none.
func (b *treeBuffer) findChild(startIndex, endIndex, dir int) int {
	pick := -1
	for i := startIndex; i != endIndex; i = int(b.buffer[i+3]) {
		pick = i
		if dir > 0 {
			break
		}
	}
	return pick
}
