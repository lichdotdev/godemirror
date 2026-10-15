// Ported from @lezer/lr 1.4.11 (src/parse.ts: LRParser, Dialect), MIT
// licensed, Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

import (
	"fmt"
	"slices"
	"strings"
)

// fileVersion is the table format @lezer/lr 1.4.11 reads.
const fileVersion = 14

// Parser holds a grammar's parse tables and parses text with them, as
// @lezer/lr's LRParser does. It is immutable and safe to share between
// goroutines.
type Parser struct {
	types         []NodeType
	minRepeatTerm int
	maxNode       int
	maxTerm       int
	states        []uint32
	data          []uint16
	gotos         []uint16
	tokenizers    []Tokenizer
	tokenPrec     int
	topRules      []TopRule
	top           TopRule
	dialects      []DialectSpec
	dialect       dialect
	dynamicPrec   map[int]int
	specialized   []int
	specializers  []func(text string, s *Stack) int // results encoded as term<<1 | extend
	context       *ContextTracker
	nest          Nest
}

// Externals are the Go ports of what a grammar takes from its own
// JavaScript module: external tokenizers and specializers by export name,
// and its @context tracker.
type Externals struct {
	Tokenizers   map[string]Tokenizer
	Specializers map[string]Specializer
	Context      *ContextTracker
}

// Specializer maps a token's text to the term it specializes to, or -1 to
// leave the token as it is. Whether that term replaces the token or is
// tried alongside it is the grammar's @external specialize or extend.
type Specializer func(text string, s *Stack) int

// New builds the parser for spec, taking the externals it names from ext.
// A missing external, a table of another format or a tag Parse rejects is
// an error.
func New(spec Spec, ext Externals) (*Parser, error) {
	if spec.Version != fileVersion {
		return nil, fmt.Errorf("parser version %d does not match runtime version %d", spec.Version, fileVersion)
	}
	types, err := nodeTypes(spec)
	if err != nil {
		return nil, err
	}
	p := &Parser{
		types:         types,
		minRepeatTerm: len(types) - spec.RepeatNodeCount,
		maxNode:       len(types) - 1,
		maxTerm:       spec.MaxTerm,
		states:        decodeArray(spec.States),
		data:          decode16(spec.StateData),
		gotos:         decode16(spec.Goto),
		tokenPrec:     spec.TokenPrec,
		topRules:      spec.TopRules,
		top:           spec.TopRules[0],
		dialects:      spec.Dialects,
		dynamicPrec:   spec.DynamicPrec,
	}
	if err := p.bindExternals(spec, ext); err != nil {
		return nil, err
	}
	p.dialect = p.parseDialect("")
	return p, nil
}

func (p *Parser) bindExternals(spec Spec, ext Externals) error {
	tokenData := decode16(spec.TokenData)
	for _, t := range spec.Tokenizers {
		tok, err := t.tokenizer(tokenData, ext)
		if err != nil {
			return err
		}
		p.tokenizers = append(p.tokenizers, tok)
	}
	for _, s := range spec.Specialized {
		get, err := s.specializer(ext)
		if err != nil {
			return err
		}
		p.specialized = append(p.specialized, s.Term)
		p.specializers = append(p.specializers, get)
	}
	if spec.Context != "" {
		if ext.Context == nil {
			return fmt.Errorf("grammar needs context tracker %q: pass it as Externals.Context", spec.Context)
		}
		p.context = ext.Context
	}
	return nil
}

func (t TokenizerSpec) tokenizer(tokenData []uint16, ext Externals) (Tokenizer, error) {
	switch {
	case t.External != "":
		tok, ok := ext.Tokenizers[t.External]
		if !ok {
			return Tokenizer{}, fmt.Errorf("grammar needs external tokenizer %q: pass it in Externals.Tokenizers", t.External)
		}
		return tok, nil
	case t.Local != nil:
		return localTokenGroup(*t.Local), nil
	}
	return tokenGroup(tokenData, t.Group), nil
}

func (s SpecializerSpec) specializer(ext Externals) (func(string, *Stack) int, error) {
	if s.External == "" {
		table := s.Table
		return func(text string, _ *Stack) int {
			if v, ok := table[text]; ok {
				return v
			}
			return -1
		}, nil
	}
	get, ok := ext.Specializers[s.External]
	if !ok {
		return nil, fmt.Errorf("grammar needs external specializer %q: pass it in Externals.Specializers", s.External)
	}
	mask := 0
	if s.Extend {
		mask = specializeExtend
	}
	return func(text string, st *Stack) int { return get(text, st)<<1 | mask }, nil
}

// Config changes what a Configure'd parser parses: Top names one of the
// grammar's top rules ("" keeps the current one), Dialect is a
// space-separated list of dialects to enable.
type Config struct {
	Top     string
	Dialect string
}

// Configure is a copy of p with c applied. An unknown top rule is an error.
func (p *Parser) Configure(c Config) (*Parser, error) {
	cp := *p
	if c.Top != "" {
		i := slices.IndexFunc(p.topRules, func(r TopRule) bool { return r.Name == c.Top })
		if i < 0 {
			return nil, fmt.Errorf("invalid top rule name %q", c.Top)
		}
		cp.top = p.topRules[i]
	}
	cp.dialect = p.parseDialect(c.Dialect)
	return &cp, nil
}

// Types are the parser's node types, indexed by id.
func (p *Parser) Types() []NodeType { return p.types }

func (p *Parser) eofTerm() int { return p.maxNode + 1 }

func (p *Parser) getGoto(state, term int, loose bool) int {
	table := p.gotos
	if term >= int(table[0]) {
		return -1
	}
	for pos := int(table[term+1]); ; {
		groupTag := int(table[pos])
		last := groupTag&1 != 0
		target := int(table[pos+1])
		pos += 2
		if last && loose {
			return target
		}
		for end := pos + groupTag>>1; pos < end; pos++ {
			if int(table[pos]) == state {
				return target
			}
		}
		if last {
			return -1
		}
	}
}

func (p *Parser) hasAction(state, terminal int) int {
	data := p.data
	for set := range 2 {
		slot := slotActions
		if set == 1 {
			slot = slotSkip
		}
		for i := p.stateSlot(state, slot); ; i += 3 {
			next := int(data[i])
			if next == seqEnd {
				if data[i+1] == seqNext {
					i = pair(data, i+2)
					next = int(data[i])
				} else if data[i+1] == seqOther {
					return pair(data, i+2)
				} else {
					break
				}
			}
			if next == terminal || next == termErr {
				return pair(data, i+1)
			}
		}
	}
	return 0
}

func (p *Parser) stateSlot(state, slot int) int {
	return int(p.states[state*stateSize+slot])
}

func (p *Parser) stateFlag(state, flag int) bool {
	return p.stateSlot(state, slotFlags)&flag > 0
}

func (p *Parser) validAction(state, action int) bool {
	_, ok := p.allActions(state, func(a int) (int, bool) { return 0, a == action })
	return ok
}

// allActions calls action for each of state's actions until it reports a
// result.
func (p *Parser) allActions(state int, action func(int) (int, bool)) (int, bool) {
	if deflt := p.stateSlot(state, slotDefaultReduce); deflt != 0 {
		if r, ok := action(deflt); ok {
			return r, true
		}
	}
	for i := p.stateSlot(state, slotActions); ; i += 3 {
		if p.data[i] == seqEnd {
			if p.data[i+1] != seqNext {
				return 0, false
			}
			i = pair(p.data, i+2)
		}
		if r, ok := action(pair(p.data, i+1)); ok {
			return r, true
		}
	}
}

// nextStates are the (term, state) pairs state can shift or goto into.
func (p *Parser) nextStates(state int) []int {
	var result []int
	for i := p.stateSlot(state, slotActions); ; i += 3 {
		if p.data[i] == seqEnd {
			if p.data[i+1] != seqNext {
				return result
			}
			i = pair(p.data, i+2)
		}
		if int(p.data[i+2])&(actionReduceFlag>>16) != 0 {
			continue
		}
		value := int(p.data[i+1])
		seen := false
		for j := 1; j < len(result); j += 2 {
			seen = seen || result[j] == value
		}
		if !seen {
			result = append(result, int(p.data[i]), value)
		}
	}
}

func (p *Parser) dynamicPrecedence(term int) int { return p.dynamicPrec[term] }

func pair(data []uint16, off int) int { return int(data[off]) | int(data[off+1])<<16 }

// dialect is which dialects a parser has enabled, and the terms the
// disabled ones alone would allow.
type dialect struct {
	flags    []bool
	disabled []bool
}

func (d dialect) allows(term int) bool { return d.disabled == nil || !d.disabled[term] }

func (p *Parser) parseDialect(enabled string) dialect {
	d := dialect{flags: make([]bool, len(p.dialects))}
	for _, part := range strings.Fields(enabled) {
		if i := slices.IndexFunc(p.dialects, func(s DialectSpec) bool { return s.Name == part }); i >= 0 {
			d.flags[i] = true
		}
	}
	for i, spec := range p.dialects {
		if d.flags[i] {
			continue
		}
		if d.disabled == nil {
			d.disabled = make([]bool, p.maxTerm+1)
		}
		for j := spec.Offset; p.data[j] != seqEnd; j++ {
			d.disabled[p.data[j]] = true
		}
	}
	return d
}
