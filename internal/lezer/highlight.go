// Ported from @lezer/highlight 1.2.5 (src/highlight.ts: Rule,
// getStyleTags, highlightTree and its HighlightBuilder), MIT licensed,
// Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

import (
	"slices"
	"strings"

	"github.com/lichdotdev/godemirror/tags"
)

// Mode is how a styleTags rule applies below its node, with the values
// @lezer/highlight gives them.
type Mode int

const (
	// ModeOpaque, a path ending in "!", styles the whole node and stops
	// matching inside it.
	ModeOpaque Mode = iota
	// ModeInherit, a path ending in "/...", adds the tags to every node
	// inside as well.
	ModeInherit
	// ModeNormal styles the node, its children matching their own rules.
	ModeNormal
)

// Rule is one styleTags rule of a node type: its tags, the names of the
// direct parents the node must have for it to apply ("" for any), and the
// texts it is limited to (nil for any).
type Rule struct {
	Tags    []*tags.Tag
	Mode    Mode
	Context []string
	Names   map[string]bool
}

// TagSet is the tags that apply to a piece of text: a node's own and the
// ones inherited from its ancestors, in that order, as the class string
// highlightTree builds lists them. TagSets from one TagSets are interned,
// so two are equal exactly when they hold the same list.
type TagSet struct {
	Tags []*tags.Tag
}

// TagSets interns TagSets. It is not safe for concurrent use.
type TagSets struct {
	byKey  map[string]*TagSet
	single map[*tags.Tag]*TagSet
	byRule map[*Rule]*TagSet
	joined map[[2]*TagSet]*TagSet
}

// NewTagSets returns an empty TagSets.
func NewTagSets() *TagSets {
	return &TagSets{
		byKey: map[string]*TagSet{}, single: map[*tags.Tag]*TagSet{},
		byRule: map[*Rule]*TagSet{}, joined: map[[2]*TagSet]*TagSet{},
	}
}

// Of is the interned set holding ts, nil for none.
func (s *TagSets) Of(ts ...*tags.Tag) *TagSet {
	if len(ts) == 0 {
		return nil
	}
	if len(ts) == 1 {
		if set, ok := s.single[ts[0]]; ok {
			return set
		}
	}
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = t.String()
	}
	key := strings.Join(names, " ")
	if set, ok := s.byKey[key]; ok {
		return set
	}
	set := &TagSet{Tags: slices.Clone(ts)}
	s.byKey[key] = set
	if len(ts) == 1 {
		s.single[ts[0]] = set
	}
	return set
}

func (s *TagSets) ofRule(r *Rule) *TagSet {
	set, ok := s.byRule[r]
	if !ok {
		set = s.Of(r.Tags...)
		s.byRule[r] = set
	}
	return set
}

// join is a's tags followed by b's.
func (s *TagSets) join(a, b *TagSet) *TagSet {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	k := [2]*TagSet{a, b}
	set, ok := s.joined[k]
	if !ok {
		set = s.Of(append(append([]*tags.Tag(nil), a.Tags...), b.Tags...)...)
		s.joined[k] = set
	}
	return set
}

// Highlight calls span, in order, for every piece of t whose TagSet is not
// empty, merging neighbors with the same set, as highlightTree does with a
// highlighter that gives every tag its own class. text is what t was
// parsed from, in UTF-16 code units, which rules with Names read.
func Highlight(t *Tree, text []uint16, sets *TagSets, span func(from, to int, set *TagSet)) {
	b := &highlightBuilder{sets: sets, span: span, text: text}
	b.highlightRange(newCursor(t), 0, t.Length(), nil)
	b.flush(t.Length())
}

type highlightBuilder struct {
	text  []uint16
	sets  *TagSets
	span  func(from, to int, set *TagSet)
	at    int
	class *TagSet
}

func (b *highlightBuilder) startSpan(at int, class *TagSet) {
	if class == b.class {
		return
	}
	b.flush(at)
	b.at = max(b.at, at)
	b.class = class
}

func (b *highlightBuilder) flush(to int) {
	if to > b.at && b.class != nil {
		b.span(b.at, to, b.class)
	}
}

// highlightRange styles the part of the cursor's node inside [from, to),
// leaving the cursor where it found it.
func (b *highlightBuilder) highlightRange(c *cursor, from, to int, inherited *TagSet) {
	start, end := c.from, c.to
	if start >= to || end <= from {
		return
	}
	class := inherited
	rule := c.styleRule(b.text)
	if rule != nil && len(rule.Tags) > 0 {
		own := b.sets.ofRule(rule)
		class = b.sets.join(class, own)
		if rule.Mode == ModeInherit {
			inherited = b.sets.join(inherited, own)
		}
	}
	b.startSpan(max(from, start), class)
	if rule != nil && rule.Mode == ModeOpaque {
		return
	}
	var mounted *mountedTree
	if c.buffer == nil {
		mounted = c.tree.tree.mounted
	}
	if mounted != nil && mounted.overlay != nil {
		b.highlightOverlay(c, from, to, inherited, class, mounted)
		return
	}
	if !c.firstChild() {
		return
	}
	if mounted != nil {
		inherited = nil
	}
	for {
		if c.to > from {
			if c.from >= to {
				break
			}
			b.highlightRange(c, from, to, inherited)
			b.startSpan(min(to, c.to), class)
		}
		if !c.nextSibling() {
			break
		}
	}
	c.parent()
}

// highlightOverlay styles a node whose mounted tree covers only its
// overlay ranges: its own children in the gaps between them, the mounted
// tree, entered with nothing inherited, inside them.
func (b *highlightBuilder) highlightOverlay(c *cursor, from, to int, inherited, class *TagSet, m *mountedTree) {
	start, end := c.from, c.to
	inner := &treeNode{tree: m.tree, from: m.overlay[0].From + start, index: -1, parent: c.tree}
	hasChild := c.firstChild()
	for i, pos := 0, start; ; i++ {
		nextPos := end
		if i < len(m.overlay) {
			nextPos = m.overlay[i].From + start
		}
		if rangeFrom, rangeTo := max(from, pos), min(to, nextPos); rangeFrom < rangeTo && hasChild {
			for c.from < rangeTo {
				b.highlightRange(c, rangeFrom, rangeTo, inherited)
				b.startSpan(min(rangeTo, c.to), class)
				if c.to >= nextPos || !c.nextSibling() {
					break
				}
			}
		}
		if i >= len(m.overlay) || nextPos > to {
			break
		}
		pos = m.overlay[i].To + start
		if pos > from {
			innerCursor := &cursor{}
			innerCursor.yieldNode(inner)
			b.highlightRange(innerCursor, max(from, nextPos), min(to, pos), nil)
			b.startSpan(min(to, pos), class)
		}
	}
	if hasChild {
		c.parent()
	}
}
