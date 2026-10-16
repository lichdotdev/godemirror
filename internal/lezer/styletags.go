// Ported from @lezer/highlight 1.2.5 (src/highlight.ts: styleTags, Rule.sort
// and ruleNodeProp's combine) and @lezer/common 1.5.3 (NodeType.match,
// NodeSet.extend), MIT licensed, Copyright (C) 2018 by Marijn Haverbeke and
// others. See LICENSE.

package lezer

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/lichdotdev/godemirror/tags"
)

// StyleSpec is one entry of a styleTags object: space-separated paths
// ("Block/...", "Annotation!", "*/ArgumentList/Identifier") and the tags
// the nodes they name take. Names, when set, limit the entry to nodes whose
// text is one of them, which styleTags has no way to say.
type StyleSpec struct {
	Selector string
	Tags     []*tags.Tag
	Names    []string
}

// StyleTags is what @lezer/highlight's styleTags makes of specs: each node
// name's rules, deepest context first. A path that does not parse is an
// error naming it.
func StyleTags(specs []StyleSpec) (map[string][]Rule, error) {
	byName := map[string][]Rule{}
	for _, spec := range specs {
		for _, part := range strings.Split(spec.Selector, " ") {
			if part == "" {
				continue
			}
			pieces, mode, err := parsePath(part)
			if err != nil {
				return nil, err
			}
			last := len(pieces) - 1
			inner := pieces[last]
			if inner == "" {
				return nil, fmt.Errorf("invalid path %q: it must end in a node name", part)
			}
			rule := Rule{Tags: spec.Tags, Mode: mode, Names: nameSet(spec.Names)}
			if last > 0 {
				rule.Context = pieces[:last]
			}
			byName[inner] = sortRule(rule, byName[inner])
		}
	}
	return byName, nil
}

func nameSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

var pathPiece = regexp.MustCompile(`^(?:"(?:[^"\\]|\\.)*?"|[^/!]+)`)

// parsePath splits a styleTags path into its node names, "" for "*", and
// reads its mode off its end.
func parsePath(part string) ([]string, Mode, error) {
	var pieces []string
	rest := part
	for pos := 0; ; {
		if rest == "..." && pos > 0 && pos+3 == len(part) {
			return pieces, ModeInherit, nil
		}
		m := pathPiece.FindString(rest)
		if m == "" {
			return nil, 0, fmt.Errorf("invalid path %q", part)
		}
		piece, err := pathName(m)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid path %q: %w", part, err)
		}
		pieces = append(pieces, piece)
		pos += len(m)
		if pos == len(part) {
			return pieces, ModeNormal, nil
		}
		next := part[pos]
		pos++
		if pos == len(part) && next == '!' {
			return pieces, ModeOpaque, nil
		}
		if next != '/' {
			return nil, 0, fmt.Errorf("invalid path %q", part)
		}
		rest = part[pos:]
	}
}

func pathName(piece string) (string, error) {
	if piece == "*" {
		return "", nil
	}
	if piece[0] == '"' {
		return strconv.Unquote(piece)
	}
	return piece, nil
}

// sortRule puts rule into rules, after the ones at least as deep, as
// Rule.sort does, but before the ones as deep with no Names.
func sortRule(rule Rule, rules []Rule) []Rule {
	i := slices.IndexFunc(rules, func(r Rule) bool { return goesBefore(rule, r) })
	if i < 0 {
		i = len(rules)
	}
	return slices.Insert(slices.Clone(rules), i, rule)
}

// goesBefore is whether rule x is tried before y: it is deeper, or as deep
// and limited to Names y is not limited to, so that a node whose text is not
// among them falls through to y.
func goesBefore(x, y Rule) bool {
	if len(x.Context) != len(y.Context) {
		return len(x.Context) > len(y.Context)
	}
	return x.Names != nil && y.Names == nil
}

// combineRules is ruleNodeProp's combine: a node type's rules a and the
// rules b added to it, deepest first, b's first among equally deep ones
// unless a's has Names b's lacks, leaving out a context-free rule of the
// mode of a context-free one just taken, as long as that one has no Names
// a node can miss.
func combineRules(a, b []Rule) []Rule {
	var out []Rule
	for len(a) > 0 || len(b) > 0 {
		var take Rule
		if len(a) == 0 || len(b) > 0 && !goesBefore(a[0], b[0]) {
			take, b = b[0], b[1:]
		} else {
			take, a = a[0], a[1:]
		}
		if n := len(out); n > 0 && out[n-1].Names == nil && out[n-1].Mode == take.Mode && take.Context == nil && out[n-1].Context == nil {
			continue
		}
		out = append(out, take)
	}
	return out
}

// WithStyleTags is p with byName's rules added to its node types' own, as
// configuring a parser with a styleTags prop does: a node type takes the
// rules of its name, or else of the first of its groups that has some.
// Trees p parsed are not reused by the new parser's incremental parses,
// since they hold p's node types.
func (p *Parser) WithStyleTags(byName map[string][]Rule) *Parser {
	cp := *p
	cp.types = slices.Clone(p.types)
	for i := range cp.types {
		rules := matchRules(&cp.types[i], byName)
		switch {
		case rules == nil:
		case cp.types[i].Rules == nil:
			// NodeSet.extend combines only with a prop the type has.
			cp.types[i].Rules = rules
		default:
			cp.types[i].Rules = combineRules(cp.types[i].Rules, rules)
		}
	}
	return &cp
}

// matchRules is NodeType.match: the rules for t's name, or else for the
// first of its groups that has some.
func matchRules(t *NodeType, byName map[string][]Rule) []Rule {
	if rules, ok := byName[t.Name]; ok {
		return rules
	}
	for _, group := range strings.Fields(t.Props["group"]) {
		if rules, ok := byName[group]; ok {
			return rules
		}
	}
	return nil
}
