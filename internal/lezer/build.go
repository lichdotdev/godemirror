// Ported from @lezer/common 1.5.3 (src/tree.ts: buildTree, balanceRange),
// MIT licensed, Copyright (C) 2018 by Marijn Haverbeke and others. See
// LICENSE. Node props (contextHash, lookAhead) only serve incremental
// parsing, which has no port, and are left out.

package lezer

import "math"

// Limits of buildTree, by their names in @lezer/common.
const (
	cutOffDepth   = 2500
	branchFactor  = 8
	maxBufferSize = defaultBufferLength
)

// bufferCursor reads a parse's post-order (type, start, end, size) quads
// from the end back.
type bufferCursor struct {
	buf []int
	pos int
}

func (c *bufferCursor) id() int    { return c.buf[c.pos-4] }
func (c *bufferCursor) start() int { return c.buf[c.pos-3] }
func (c *bufferCursor) end() int   { return c.buf[c.pos-2] }
func (c *bufferCursor) size() int  { return c.buf[c.pos-1] }
func (c *bufferCursor) next()      { c.pos -= 4 }

type treeBuilder struct {
	cursor        bufferCursor
	types         []NodeType
	minRepeatType int
	sizes         map[*Tree]int
}

// buildTree builds the tree a parse's buffer describes under a top node
// of type top, as Tree.build does.
func buildTree(p *Parser, buf []int, top, start, length int) *Tree {
	b := &treeBuilder{cursor: bufferCursor{buf, len(buf)}, types: p.types, minRepeatType: p.minRepeatTerm, sizes: map[*Tree]int{}}
	var children []treeChild
	var positions []int
	for b.cursor.pos > 0 {
		b.takeNode(start, 0, &children, &positions, -1, 0)
	}
	reverse(children, positions)
	return &Tree{typ: &p.types[top], children: children, positions: positions, length: length}
}

func reverse(children []treeChild, positions []int) {
	for i, j := 0, len(children)-1; i < j; i, j = i+1, j-1 {
		children[i], children[j] = children[j], children[i]
		positions[i], positions[j] = positions[j], positions[i]
	}
}

func (b *treeBuilder) takeNode(parentStart, minPos int, children *[]treeChild, positions *[]int, inRepeat, depth int) {
	c := &b.cursor
	id, start, end, size := c.id(), c.start(), c.end(), c.size()
	if size < 0 {
		// A context hash or lookahead record; reused nodes never occur.
		c.next()
		return
	}
	if end-start <= maxBufferSize {
		if found, ok := b.findBufferSize(c.pos-minPos, inRepeat); ok {
			data := make([]uint16, found.size-found.skip)
			endPos, index := c.pos-found.size, len(data)
			for c.pos > endPos {
				index = b.copyToBuffer(found.start, data, index)
			}
			*children = append(*children, treeChild{buffer: &treeBuffer{buffer: data, length: end - found.start, types: b.types}})
			*positions = append(*positions, found.start-parentStart)
			return
		}
	}
	*children = append(*children, treeChild{tree: b.takeTree(id, start, end, size, depth)})
	*positions = append(*positions, start-parentStart)
}

// takeTree makes the node at the cursor a Tree, its children taken from the
// records it covers, a repeat node's long runs balanced.
func (b *treeBuilder) takeTree(id, start, end, size, depth int) *Tree {
	c := &b.cursor
	endPos := c.pos - size
	c.next()
	var children []treeChild
	var positions []int
	localInRepeat := -1
	if id >= b.minRepeatType {
		localInRepeat = id
	}
	lastGroup, lastEnd := 0, end
	for c.pos > endPos {
		switch {
		case localInRepeat >= 0 && c.id() == localInRepeat && c.size() >= 0:
			if c.end() <= lastEnd-maxBufferSize {
				b.makeRepeatLeaf(&children, &positions, start, lastGroup, c.end(), lastEnd, localInRepeat)
				lastGroup, lastEnd = len(children), c.end()
			}
			c.next()
		case depth > cutOffDepth:
			b.takeFlatNode(start, endPos, &children, &positions)
		default:
			b.takeNode(start, endPos, &children, &positions, localInRepeat, depth+1)
		}
	}
	if localInRepeat >= 0 && lastGroup > 0 && lastGroup < len(children) {
		b.makeRepeatLeaf(&children, &positions, start, lastGroup, start, lastEnd, localInRepeat)
	}
	reverse(children, positions)
	typ := &b.types[id]
	if localInRepeat > -1 && lastGroup > 0 {
		return b.balanceRange(typ, children, positions, 0, len(children), 0, end-start, makeBalanced(typ), makeBalanced(typ))
	}
	return &Tree{typ: typ, children: children, positions: positions, length: end - start}
}

// takeFlatNode collects the leaves under a tree too deep to recurse into
// as one flat buffer.
func (b *treeBuilder) takeFlatNode(parentStart, minPos int, children *[]treeChild, positions *[]int) {
	c := &b.cursor
	var nodes []int
	stopAt := -1
	for c.pos > minPos {
		id, start, end, size := c.id(), c.start(), c.end(), c.size()
		if size > 4 {
			c.next()
			continue
		}
		if stopAt > -1 && start < stopAt {
			break
		}
		if stopAt < 0 {
			stopAt = end - maxBufferSize
		}
		nodes = append(nodes, id, start, end)
		c.next()
	}
	if len(nodes) == 0 {
		return
	}
	buffer := make([]uint16, len(nodes)/3*4)
	start := nodes[len(nodes)-2]
	for i, j := len(nodes)-3, 0; i >= 0; i -= 3 {
		buffer[j], buffer[j+1], buffer[j+2] = uint16(nodes[i]), uint16(nodes[i+1]-start), uint16(nodes[i+2]-start)
		j += 4
		buffer[j-1] = uint16(j)
	}
	*children = append(*children, treeChild{buffer: &treeBuffer{buffer: buffer, length: nodes[2] - start, types: b.types}})
	*positions = append(*positions, start-parentStart)
}

// makeRepeatLeaf groups the children from index i on under one node of the
// repeat type.
func (b *treeBuilder) makeRepeatLeaf(children *[]treeChild, positions *[]int, base, i, from, to, typ int) {
	var local []treeChild
	var localPositions []int
	for len(*children) > i {
		last := len(*children) - 1
		local = append(local, (*children)[last])
		localPositions = append(localPositions, (*positions)[last]+base-from)
		*children, *positions = (*children)[:last], (*positions)[:last]
	}
	*children = append(*children, treeChild{tree: &Tree{typ: &b.types[typ], children: local, positions: localPositions, length: to - from}})
	*positions = append(*positions, from-base)
}

type bufferSize struct{ size, start, skip int }

// findBufferSize scans back for previous siblings that fit together in a
// TreeBuffer. Nested repeat nodes of type inRepeat do not count as nesting,
// but the end must then fall at maxSize or before such a node.
func (b *treeBuilder) findBufferSize(maxSize, inRepeat int) (bufferSize, bool) {
	fork := b.cursor
	size, start, skip := 0, 0, 0
	minStart := fork.end() - maxBufferSize
	result := bufferSize{}
scan:
	for minPos := fork.pos - maxSize; fork.pos > minPos; {
		nodeSize := fork.size()
		if fork.id() == inRepeat && nodeSize >= 0 {
			result = bufferSize{size, start, skip}
			skip += 4
			size += 4
			fork.next()
			continue
		}
		startPos := fork.pos - nodeSize
		if nodeSize < 0 || startPos < minPos || fork.start() < minStart {
			break
		}
		localSkipped := 0
		if fork.id() >= b.minRepeatType {
			localSkipped = 4
		}
		nodeStart := fork.start()
		fork.next()
		for fork.pos > startPos {
			if fork.size() < 0 {
				if fork.size() != recordContextChange && fork.size() != recordLookAhead {
					break scan
				}
				localSkipped += 4
			} else if fork.id() >= b.minRepeatType {
				localSkipped += 4
			}
			fork.next()
		}
		start, size, skip = nodeStart, size+nodeSize, skip+localSkipped
	}
	if inRepeat < 0 || size == maxSize {
		result = bufferSize{size, start, skip}
	}
	return result, result.size > 4
}

// copyToBuffer copies the node at the cursor, dropping repeat nodes, into
// buffer before index, returning the new index.
func (b *treeBuilder) copyToBuffer(bufferStart int, buffer []uint16, index int) int {
	c := &b.cursor
	id, start, end, size := c.id(), c.start(), c.end(), c.size()
	c.next()
	if size < 0 || id >= b.minRepeatType {
		return index
	}
	startIndex := index
	if size > 4 {
		for endPos := c.pos - (size - 4); c.pos > endPos; {
			index = b.copyToBuffer(bufferStart, buffer, index)
		}
	}
	index -= 4
	buffer[index], buffer[index+1], buffer[index+2], buffer[index+3] = uint16(id), uint16(start-bufferStart), uint16(end-bufferStart), uint16(startIndex)
	return index
}

func makeBalanced(typ *NodeType) func([]treeChild, []int, int) *Tree {
	return func(children []treeChild, positions []int, length int) *Tree {
		if last := len(children) - 1; last == 0 && children[0].tree != nil && children[0].tree.typ == typ && children[0].tree.length == length {
			return children[0].tree
		}
		return &Tree{typ: typ, children: children, positions: positions, length: length}
	}
}

func (b *treeBuilder) nodeSize(balanceType *NodeType, c treeChild) int {
	if !balanceType.Anonymous() || c.buffer != nil || c.tree.typ != balanceType {
		return 1
	}
	if size, ok := b.sizes[c.tree]; ok {
		return size
	}
	size := 1
	for _, child := range c.tree.children {
		if child.tree == nil || child.tree.typ != balanceType {
			size = 1
			break
		}
		size += b.nodeSize(balanceType, child)
	}
	b.sizes[c.tree] = size
	return size
}

// balanceRange groups children[from:to] under nodes of balanceType, so no
// node gets more than about branchFactor children.
func (b *treeBuilder) balanceRange(balanceType *NodeType, children []treeChild, positions []int, from, to, start, length int, mkTop, mkTree func([]treeChild, []int, int) *Tree) *Tree {
	total := 0
	for i := from; i < to; i++ {
		total += b.nodeSize(balanceType, children[i])
	}
	maxChild := int(math.Ceil(float64(total) * 1.5 / branchFactor))
	d := &divider{b: b, balanceType: balanceType, maxChild: maxChild, start: start, mkTree: mkTree}
	d.divide(children, positions, from, to, 0)
	if mkTop == nil {
		mkTop = mkTree
	}
	return mkTop(d.children, d.positions, length)
}

type divider struct {
	b           *treeBuilder
	balanceType *NodeType
	maxChild    int
	start       int
	mkTree      func([]treeChild, []int, int) *Tree
	children    []treeChild
	positions   []int
}

func (d *divider) divide(children []treeChild, positions []int, from, to, offset int) {
	for i := from; i < to; {
		groupFrom, groupStart := i, positions[i]
		groupSize := d.b.nodeSize(d.balanceType, children[i])
		for i++; i < to; i++ {
			nextSize := d.b.nodeSize(d.balanceType, children[i])
			if groupSize+nextSize >= d.maxChild {
				break
			}
			groupSize += nextSize
		}
		switch {
		case i == groupFrom+1 && groupSize > d.maxChild:
			only := children[groupFrom].tree
			d.divide(only.children, only.positions, 0, len(only.children), positions[groupFrom]+offset)
			continue
		case i == groupFrom+1:
			d.children = append(d.children, children[groupFrom])
		default:
			length := positions[i-1] + children[i-1].length() - groupStart
			d.children = append(d.children, treeChild{tree: d.b.balanceRange(d.balanceType, children, positions, groupFrom, i, groupStart, length, nil, d.mkTree)})
		}
		d.positions = append(d.positions, groupStart+offset-d.start)
	}
}
