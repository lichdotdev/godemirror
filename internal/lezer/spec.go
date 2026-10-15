// Ported from @lezer/lr 1.4.11 (src/parse.ts, LRParser's constructor and
// deserialize) and @lezer/common 1.5.3 (NodeType), MIT licensed, Copyright (C)
// 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

import (
	"fmt"
	"strings"

	"github.com/lichdotdev/godemirror/tags"
)

// Spec is a grammar's parse tables, as lezer-generator writes them for
// LRParser.deserialize, plus the styleTags rules its language package
// attaches. internal/lezer/gen writes one per grammar.
type Spec struct {
	Version         int
	States          string
	StateData       string
	Goto            string
	NodeNames       string
	MaxTerm         int
	RepeatNodeCount int
	NodeProps       map[string]map[int]string
	SkippedNodes    []int
	TokenData       string
	Tokenizers      []TokenizerSpec
	TopRules        []TopRule
	Dialects        []DialectSpec
	DynamicPrec     map[int]int
	Specialized     []SpecializerSpec
	TokenPrec       int
	// Context names the grammar's @context tracker, "" for none.
	Context string
	// Styles are each node type's styleTags rules, already combined and
	// sorted the way @lezer/highlight's ruleNodeProp leaves them.
	Styles map[int][]RuleSpec
}

// TokenizerSpec is one entry of a grammar's tokenizers: a token group of
// its TokenData, a local token group with its own data, or an external
// tokenizer by its export name.
type TokenizerSpec struct {
	Group    int
	Local    *LocalGroupSpec
	External string
}

// LocalGroupSpec is a LocalTokenGroup's constructor arguments.
type LocalGroupSpec struct {
	Data      string
	PrecTable int
	ElseToken int // -1 for none
}

// TopRule is one of a grammar's top rules, in the order the grammar lists
// them: the first is the default.
type TopRule struct {
	Name        string
	State, Term int
}

// DialectSpec is a dialect and the offset in the state data of the terms
// it alone enables.
type DialectSpec struct {
	Name   string
	Offset int
}

// SpecializerSpec is a @specialize or @extend on Term: a table from token
// text to the encoded result (term<<1, |1 for an extension), or an
// external specializer by its export name.
type SpecializerSpec struct {
	Term     int
	Table    map[string]int
	External string
	Extend   bool
}

// RuleSpec is one styleTags rule as lezer-generator's caller wrote it:
// tags as Tag.toString prints them.
type RuleSpec struct {
	Tags    []string
	Mode    Mode
	Context []string
}

// NodeType is a kind of node in a syntax tree.
type NodeType struct {
	Name  string
	ID    int
	Props map[string]string
	Top   bool
	Error bool
	// Skipped marks nodes the grammar's @skip produces, like comments.
	Skipped bool
	// Rules are the styleTags rules for the type, deepest context first.
	Rules []Rule
}

// Anonymous reports whether t has no name, as the repeat nodes
// lezer-generator adds for a `*` or `+`: a cursor walks through an
// anonymous Tree to its children.
func (t *NodeType) Anonymous() bool { return t.Name == "" }

// nodeTypes builds the node types of spec, the way LRParser's constructor
// does, with each type's styleTags rules resolved to tags.
func nodeTypes(spec Spec) ([]NodeType, error) {
	names := strings.Split(spec.NodeNames, " ")
	for range spec.RepeatNodeCount {
		names = append(names, "")
	}
	types := make([]NodeType, len(names))
	for i, name := range names {
		types[i] = NodeType{Name: name, ID: i, Error: i == 0, Props: map[string]string{}}
	}
	for _, top := range spec.TopRules {
		types[top.Term].Top = true
	}
	for _, id := range spec.SkippedNodes {
		types[id].Skipped = true
	}
	for prop, values := range spec.NodeProps {
		for id, v := range values {
			types[id].Props[prop] = v
		}
	}
	for id, rules := range spec.Styles {
		resolved, err := resolveRules(rules)
		if err != nil {
			return nil, fmt.Errorf("styles of %s: %w", types[id].Name, err)
		}
		types[id].Rules = resolved
	}
	return types, nil
}

func resolveRules(specs []RuleSpec) ([]Rule, error) {
	rules := make([]Rule, len(specs))
	for i, s := range specs {
		rules[i] = Rule{Mode: s.Mode, Context: s.Context}
		for _, name := range s.Tags {
			t, err := tags.Parse(name)
			if err != nil {
				return nil, err
			}
			rules[i].Tags = append(rules[i].Tags, t)
		}
	}
	return rules, nil
}
