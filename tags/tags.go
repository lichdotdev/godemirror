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
	"fmt"
	"iter"
	"slices"
	"strings"
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

// standard and modifiersByName are the standard tags and modifiers by the
// names String prints, and modifierNames the modifiers in the order they are
// defined, for Parse. All three are filled while the package initializes and
// only read afterwards.
var (
	standard        = map[string]*Tag{}
	modifiersByName = map[string]func(*Tag) *Tag{}
	modifierNames   []string
)

// Parse is the tag whose String is expr: the format @lezer/highlight's
// Tag.toString prints, a standard tag's name ("variableName") or a modifier
// applied to a tag ("function(definition(variableName))"), modifiers in any
// order. It is how a theme written down outside Go, in JSON or TOML, names the
// tags it styles. An unknown tag or modifier name, an unbalanced parenthesis
// or text after the closing one is an error saying what was expected. Parse
// is safe to call from several goroutines.
func Parse(expr string) (*Tag, error) {
	open := strings.IndexByte(expr, '(')
	if open < 0 {
		return parseName(expr)
	}
	name := expr[:open]
	modify, ok := modifiersByName[name]
	if !ok {
		return nil, fmt.Errorf("%q in %q is not a tag modifier: want one of %s", name, expr, strings.Join(modifierNames, ", "))
	}
	end := closingParen(expr, open)
	if end < 0 {
		return nil, fmt.Errorf("%q opens a parenthesis it never closes", expr)
	}
	if end != len(expr)-1 {
		return nil, fmt.Errorf("%q has %q after the parenthesis that closes it: want nothing there", expr, expr[end+1:])
	}
	t, err := Parse(expr[open+1 : end])
	if err != nil {
		return nil, err
	}
	return modify(t), nil
}

// parseName is the standard tag named name.
func parseName(name string) (*Tag, error) {
	if strings.IndexByte(name, ')') >= 0 {
		return nil, fmt.Errorf("%q closes a parenthesis it never opened", name)
	}
	t, ok := standard[name]
	if !ok {
		return nil, fmt.Errorf("%q is not a highlighting tag: want a standard tag name such as \"variableName\"", name)
	}
	return t, nil
}

// closingParen is the index of the parenthesis closing the one at open in
// expr, -1 when none does.
func closingParen(expr string, open int) int {
	depth := 0
	for i := open; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func define(name string, parent *Tag) *Tag {
	t := &Tag{name: name}
	standard[name] = t
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
	apply := func(t *Tag) *Tag {
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
	modifiersByName[name] = apply
	modifierNames = append(modifierNames, name)
	return apply
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
