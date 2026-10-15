// Package tags is CodeMirror's vocabulary of highlighting tags, the one
// @lezer/highlight defines: a closed set of tags a highlighter maps to
// styles, arranged so that a style for a general tag covers its subtags.
// godemirror tags every token chroma reads with one of them, and a
// HighlightStyle styles them the way CodeMirror's does.
//
// A tag's Set is the tag and every tag it falls back to, most specific
// first: LineComment falls back to Comment, Definition(VariableName) to
// VariableName and then to Name. A style for any tag in the set applies,
// the most specific one winning.
package tags

import (
	"iter"
	"slices"
	"sync"
)

// Tag is a highlighting category. Tags are compared by identity: the same
// modifiers applied to the same tag, in any order, give the same *Tag.
type Tag struct {
	name     string
	set      []*Tag
	base     *Tag
	modified []*modifier
}

// Set yields t and every tag it falls back to, in decreasing specificity,
// as @lezer/highlight's tag.set lists them.
func (t *Tag) Set() iter.Seq[*Tag] {
	return slices.Values(t.set)
}

// String is the tag as @lezer/highlight prints it, as in
// "function(definition(variableName))".
func (t *Tag) String() string {
	name := t.name
	for _, m := range t.modified {
		name = m.name + "(" + name + ")"
	}
	return name
}

func define(name string, parent *Tag) *Tag {
	t := &Tag{name: name}
	t.set = append(t.set, t)
	if parent != nil {
		t.set = append(t.set, parent.set...)
	}
	return t
}

type modifier struct {
	name      string
	id        int
	instances []*Tag
}

// modifiers guards every modifier's instances: a modified tag is made the
// first time it is asked for, from whichever goroutine asks.
var modifiers sync.Mutex

var nextModifierID int

func defineModifier(name string) func(*Tag) *Tag {
	m := &modifier{name: name, id: nextModifierID}
	nextModifierID++
	return func(t *Tag) *Tag {
		modifiers.Lock()
		defer modifiers.Unlock()
		if slices.Contains(t.modified, m) {
			return t
		}
		base := t.base
		if base == nil {
			base = t
		}
		mods := append(slices.Clone(t.modified), m)
		slices.SortFunc(mods, func(a, b *modifier) int { return a.id - b.id })
		return modified(base, mods)
	}
}

// modified is base with mods applied, mods sorted by id. Its set holds, for
// each unmodified tag in base's set, that tag under every subset of mods,
// larger subsets first: the order @lezer/highlight's Modifier.get builds.
func modified(base *Tag, mods []*modifier) *Tag {
	if len(mods) == 0 {
		return base
	}
	for _, t := range mods[0].instances {
		if t.base == base && slices.Equal(t.modified, mods) {
			return t
		}
	}
	t := &Tag{name: base.name, base: base, modified: mods}
	for _, m := range mods {
		m.instances = append(m.instances, t)
	}
	configs := powerSet(mods)
	for _, parent := range base.set {
		if len(parent.modified) > 0 {
			continue
		}
		for _, config := range configs {
			t.set = append(t.set, modified(parent, config))
		}
	}
	return t
}

// powerSet is every subset of mods, larger first, keeping the order
// @lezer/highlight's powerSet gives subsets of the same size.
func powerSet(mods []*modifier) [][]*modifier {
	sets := [][]*modifier{nil}
	for _, m := range mods {
		for _, s := range sets[:len(sets):len(sets)] {
			sets = append(sets, append(slices.Clone(s), m))
		}
	}
	slices.SortStableFunc(sets, func(a, b []*modifier) int { return len(b) - len(a) })
	return sets
}
