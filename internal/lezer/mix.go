// Ported from @lezer/common 1.5.3 (src/mix.ts: parseMixed, MixedParse's
// startInner, checkCover, sliceBuf, materialize, punchRanges), MIT
// licensed, Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.
// stopAt and overlays built up node by node by a predicate have no port,
// and nor has NestedParse.bracketed, which only the enter methods read.

package lezer

import (
	"fmt"
	"slices"
	"unicode/utf16"
)

// NestedParse says how another parser reads a node's content.
type NestedParse struct {
	Parser *Parser
	// Overlay, unless nil, are the parts of the input the parser reads, as
	// absolute positions inside the node, and the node keeps its own tree
	// around them. A nil Overlay parses the whole node, and its tree
	// replaces the node's.
	Overlay []Range
}

// Nest is what @lezer/common's parseMixed takes: for a node of the outer
// parse, the parse of its content, or nil to leave it as it is.
type Nest func(n *Node, in Input) *NestedParse

// Input is the text a parse reads, for a Nest to look at.
type Input struct{ units []uint16 }

// Read is the text of [from, to), in UTF-16 code units, clipped to the
// input as JavaScript's String.slice clips it, so "" when to <= from.
func (in Input) Read(from, to int) string {
	to = min(to, len(in.units))
	if from >= to {
		return ""
	}
	return string(utf16.Decode(in.units[from:to]))
}

// WithNest is a copy of p whose parses run nest over every node of the tree
// they made and mount the nested parses it asks for, as LRParser.configure
// with a wrap of parseMixed(nest) does. A nest p had is replaced.
func (p *Parser) WithNest(nest Nest) *Parser {
	cp := *p
	cp.nest = nest
	return &cp
}

// mountedTree is NodeProp.mounted: the tree another parser made of a node's
// text. Without an overlay it replaces the node; with one, it covers only
// the overlay's ranges, relative to the node's start.
type mountedTree struct {
	tree    *Tree
	overlay []Range
	parser  *Parser
}

type innerParse struct {
	parser    *Parser
	ranges    []Range
	overlay   []Range
	target    *Tree
	fragments []TreeFragment
}

// coveredRanges are the ranges an overlay parse took, which nodes inside
// them no longer nest into, depth levels up the scan.
type coveredRanges struct {
	ranges []Range
	depth  int
	prev   *coveredRanges
}

// mountNested scans base, the tree parsing ranges of input made, for the
// nodes nest has another parser read, parses them and mounts the results.
// A node the parse reused from fragments keeps the trees mounted on it,
// and a new nested parse reuses what it can of the trees mounted where it
// is in the fragments.
func mountNested(base *Tree, nest Nest, input []uint16, ranges []Range, fragments []TreeFragment) {
	var inner []innerParse
	fc := newMixFragmentCursor(fragments)
	c := &scanCursor{}
	c.yieldNode(&treeNode{tree: base, from: ranges[0].From})
	var covered *coveredRanges
	for {
		enter := true
		if fc.hasNode(c.from, c.to, c.treeOrNil()) {
			enter = false
		} else if cover := coverOf(covered, c.from, c.to); cover != coverNone {
			enter = cover != coverFull
		} else if n := nestAt(c, nest, input); n != nil {
			if c.buffer != nil {
				c.materialize()
				if covered != nil {
					covered.depth++
				}
			}
			parse := newInnerParse(n, ranges, c.from, c.to, c.tree.tree)
			if len(parse.ranges) > 0 {
				parse.fragments = enterFragments(fc.findMounts(c.from, n.Parser), parse.ranges)
			}
			if len(parse.ranges) > 0 || n.Overlay == nil {
				inner = append(inner, parse)
			}
			if n.Overlay == nil {
				enter = false
			} else if len(parse.ranges) > 0 {
				covered = &coveredRanges{ranges: parse.ranges, prev: covered}
			}
		}
		if enter && c.firstChild() {
			if covered != nil {
				covered.depth++
			}
		} else if !c.leave(&covered) {
			break
		}
	}
	for _, parse := range inner {
		parse.run(input)
	}
}

// nestAt is what nest asks for at the cursor's node, nil for nothing or
// for an empty node with an overlay.
func nestAt(c *scanCursor, nest Nest, input []uint16) *NestedParse {
	if c.typ.Anonymous() {
		return nil
	}
	n := nest(c.node(), Input{input})
	if n == nil || (c.from == c.to && n.Overlay != nil) {
		return nil
	}
	return n
}

func newInnerParse(n *NestedParse, outer []Range, from, to int, target *Tree) innerParse {
	asked := n.Overlay
	if asked == nil && from < to {
		asked = []Range{{from, to}}
	}
	ranges := punchRanges(outer, asked)
	for _, r := range ranges {
		if r.From >= r.To {
			panic(fmt.Sprintf("lezer: invalid inner parse ranges given: %v", ranges))
		}
	}
	parse := innerParse{parser: n.Parser, ranges: ranges, target: target}
	if n.Overlay != nil {
		for _, r := range n.Overlay {
			parse.overlay = append(parse.overlay, Range{r.From - from, r.To - from})
		}
	}
	return parse
}

func (p innerParse) run(input []uint16) {
	var tree *Tree
	if len(p.ranges) > 0 {
		tree = p.parser.ParseUnits(input, p.ranges, p.fragments)
	} else {
		tree = p.parser.Parse("")
	}
	p.target.mounted = &mountedTree{tree: tree, overlay: p.overlay, parser: p.parser}
}

// leave moves the cursor to the next node after the current one and its
// children, dropping the covered ranges it climbs out of. It reports false
// at the end of the tree.
func (c *scanCursor) leave(covered **coveredRanges) bool {
	for !c.nextSibling() {
		if !c.parent() {
			return false
		}
		if cv := *covered; cv != nil {
			cv.depth--
			if cv.depth == 0 {
				*covered = cv.prev
			}
		}
	}
	return true
}

// How much of a node a covered range takes.
const (
	coverNone = iota
	coverPartial
	coverFull
)

func coverOf(covered *coveredRanges, from, to int) int {
	if covered == nil {
		return coverNone
	}
	for _, r := range covered.ranges {
		if r.From >= to {
			break
		}
		if r.To > from {
			if r.From <= from && r.To >= to {
				return coverFull
			}
			return coverPartial
		}
	}
	return coverNone
}

// punchRanges is ranges without the gaps between the outer ranges.
func punchRanges(outer, ranges []Range) []Range {
	var copied []Range
	current := ranges
	for i, j := 1, 0; i < len(outer); i++ {
		gapFrom, gapTo := outer[i-1].To, outer[i].From
		for ; j < len(current); j++ {
			r := current[j]
			if r.From >= gapTo {
				break
			}
			if r.To <= gapFrom {
				continue
			}
			if copied == nil {
				copied = slices.Clone(ranges)
				current = copied
			}
			switch {
			case r.From < gapFrom:
				copied[j] = Range{r.From, gapFrom}
				if r.To > gapTo {
					copied = slices.Insert(copied, j+1, Range{gapTo, r.To})
				}
			case r.To > gapTo:
				copied[j] = Range{gapTo, r.To}
				j--
			default:
				copied = slices.Delete(copied, j, j+1)
				j--
			}
			current = copied
		}
	}
	return current
}
