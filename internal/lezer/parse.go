// Ported from @lezer/lr 1.4.11 (src/parse.ts: Parse), MIT licensed,
// Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

import (
	"slices"
	"unicode/utf16"
)

// parse is one run of a Parser over an input, reusing the nodes of an
// earlier tree's fragments where it can.
type parse struct {
	parser                *Parser
	ranges                []Range
	recovering            int
	minStackPos           int
	lastBigReductionStart int
	lastBigReductionSize  int
	bigReductionCount     int
	stream                *InputStream
	tokens                *tokenCache
	topTerm               int
	stacks                []*Stack
	fragments             *fragmentCursor
	reused                []*Tree
}

// Parse parses text whole and returns its tree. Positions in the tree are
// UTF-16 code units, as in JavaScript.
func (p *Parser) Parse(text string) *Tree {
	return p.ParseUnits(utf16.Encode([]rune(text)), nil, nil)
}

// ParseUnits parses the ranges of input, nil ranges meaning all of it,
// reusing what it can of fragments, the pieces of earlier trees
// ApplyChanges left: an incremental parse, as Parser.parse with fragments.
// A reused node is shared with the tree it came from, and a nested parse
// may store its tree on it, so no earlier tree may be read while this
// runs.
func (p *Parser) ParseUnits(input []uint16, ranges []Range, fragments []TreeFragment) *Tree {
	if len(ranges) == 0 {
		ranges = []Range{{0, len(input)}}
	}
	ps := &parse{parser: p, ranges: ranges, lastBigReductionStart: -1, topTerm: p.top.Term}
	ps.stream = newInputStream(input, ranges)
	ps.tokens = newTokenCache(p, ps.stream)
	ps.stacks = []*Stack{startStack(ps, p.top.State, ranges[0].From)}
	ps.minStackPos = ranges[0].From
	if len(fragments) > 0 && ps.stream.end-ranges[0].From > defaultBufferLength*4 {
		ps.fragments = newFragmentCursor(fragments, p.minRepeatTerm)
	}
	for {
		if t := ps.advance(); t != nil {
			if p.nest != nil {
				mountNested(t, p.nest, input, ranges, fragments)
			}
			return t
		}
	}
}

// advance moves every stack at the lowest position forward, starting error
// recovery when none can. It returns the tree once the parse is done.
func (ps *parse) advance() *Tree {
	stacks, pos := ps.stacks, ps.minStackPos
	var newStacks, stopped []*Stack
	var stoppedTokens []int
	ps.stacks = nil
	ps.cutBigReductions(stacks)
	for i := 0; i < len(stacks); i++ {
		stack := stacks[i]
		for {
			ps.tokens.mainToken = nil
			if stack.pos > pos {
				newStacks = append(newStacks, stack)
			} else if ps.advanceStack(stack, &newStacks, &stacks) {
				continue
			} else {
				stopped = append(stopped, stack)
				tok := ps.tokens.getMainToken(stack)
				stoppedTokens = append(stoppedTokens, tok.value, tok.end)
			}
			break
		}
	}
	if len(newStacks) == 0 {
		if finished := findFinished(stopped); finished != nil {
			return ps.stackToTree(finished)
		}
		if ps.recovering == 0 {
			ps.recovering = recDistance
		}
	}
	if ps.recovering != 0 && len(stopped) > 0 {
		if finished := ps.runRecovery(stopped, stoppedTokens, &newStacks); finished != nil {
			return ps.stackToTree(finished.forceAll())
		}
	}
	newStacks = ps.prune(newStacks, pos)
	ps.stacks = newStacks
	ps.minStackPos = newStacks[0].pos
	for _, s := range newStacks[1:] {
		ps.minStackPos = min(ps.minStackPos, s.pos)
	}
	return nil
}

// cutBigReductions forces a lone stack out of a production that has seen
// many big reductions from the same start, before it builds a tree too
// deep to walk.
func (ps *parse) cutBigReductions(stacks []*Stack) {
	if ps.bigReductionCount <= recMaxLeftAssocReduction || len(stacks) != 1 {
		return
	}
	s := stacks[0]
	for s.forceReduce() && len(s.stack) > 0 && s.stack[len(s.stack)-2] >= ps.lastBigReductionStart {
	}
	ps.bigReductionCount, ps.lastBigReductionSize = 0, 0
}

// prune caps the stacks kept: fewer while recovering, and without
// duplicates or long-running twins otherwise.
func (ps *parse) prune(newStacks []*Stack, pos int) []*Stack {
	byScore := func(a, b *Stack) int { return b.score - a.score }
	if ps.recovering != 0 {
		maxRemaining := ps.recovering * recMaxRemainingPerStep
		if ps.recovering == 1 {
			maxRemaining = 1
		}
		if len(newStacks) > maxRemaining {
			slices.SortStableFunc(newStacks, byScore)
			newStacks = newStacks[:maxRemaining]
		}
		if slices.ContainsFunc(newStacks, func(s *Stack) bool { return s.reducePos > pos }) {
			ps.recovering--
		}
		return newStacks
	}
	if len(newStacks) <= 1 {
		return newStacks
	}
	newStacks = dropTwins(newStacks)
	if len(newStacks) > recMaxStackCount {
		slices.SortStableFunc(newStacks, byScore)
		newStacks = newStacks[:recMaxStackCount]
	}
	return newStacks
}

// dropTwins keeps one of every two stacks in the same state, or that have
// both run long without splitting: the higher scoring, then the longer.
func dropTwins(stacks []*Stack) []*Stack {
outer:
	for i := 0; i < len(stacks)-1; i++ {
		stack := stacks[i]
		for j := i + 1; j < len(stacks); j++ {
			other := stacks[j]
			longRunning := len(stack.buffer) > recMinBufferLengthPrune && len(other.buffer) > recMinBufferLengthPrune
			if !stack.sameState(other) && !longRunning {
				continue
			}
			diff := stack.score - other.score
			if diff == 0 {
				diff = len(stack.buffer) - len(other.buffer)
			}
			if diff > 0 {
				stacks = slices.Delete(stacks, j, j+1)
				j--
			} else {
				stacks = slices.Delete(stacks, i, i+1)
				i--
				continue outer
			}
		}
	}
	return stacks
}

// advanceStack applies the actions the next token allows. Stacks split off
// by ambiguous actions go to stacks when they moved forward and to split
// otherwise; with nil split, only the first action is taken.
func (ps *parse) advanceStack(stack *Stack, stacks, split *[]*Stack) bool {
	p := ps.parser
	if ps.fragments != nil && ps.reuseAt(stack) {
		return true
	}
	if defaultReduce := p.stateSlot(stack.state, slotDefaultReduce); defaultReduce > 0 {
		stack.reduce(defaultReduce)
		return true
	}
	if len(stack.stack) >= recCutDepth {
		for len(stack.stack) > recCutTo && stack.forceReduce() {
		}
	}
	start := stack.pos
	actions := ps.tokens.getActions(stack)
	for i := 0; i < len(actions); {
		action, term, end := actions[i], actions[i+1], actions[i+2]
		i += 3
		last := i == len(actions) || split == nil
		localStack := stack
		if !last {
			localStack = stack.split()
		}
		at := localStack.pos
		if main := ps.tokens.mainToken; main != nil {
			at = main.start
		}
		localStack.apply(action, term, at, end)
		switch {
		case last:
			return true
		case localStack.pos > start:
			*stacks = append(*stacks, localStack)
		default:
			*split = append(*split, localStack)
		}
	}
	return false
}

// advanceFully advances stack until it moves forward, then adds it to
// newStacks. It reports false when the stack got stuck instead.
func (ps *parse) advanceFully(stack *Stack, newStacks *[]*Stack) bool {
	pos := stack.pos
	for {
		if !ps.advanceStack(stack, nil, nil) {
			return false
		}
		if stack.pos > pos {
			pushStackDedup(stack, newStacks)
			return true
		}
	}
}

func (ps *parse) runRecovery(stacks []*Stack, tokens []int, newStacks *[]*Stack) *Stack {
	var finished *Stack
	restarted := false
	for i, stack := range stacks {
		token, tokenEnd := tokens[i<<1], tokens[i<<1+1]
		if stack.deadEnd() {
			if restarted {
				continue
			}
			restarted = true
			stack.restart()
			if ps.advanceFully(stack, newStacks) {
				continue
			}
		}
		force := stack.split()
		for j := 0; j < recForceReduceLimit && force.forceReduce(); j++ {
			if ps.advanceFully(force, newStacks) {
				break
			}
		}
		for _, insert := range stack.recoverByInsert(token) {
			ps.advanceFully(insert, newStacks)
		}
		if ps.stream.end > stack.pos {
			if tokenEnd == stack.pos {
				tokenEnd++
				token = termErr
			}
			stack.recoverByDelete(token, tokenEnd)
			pushStackDedup(stack, newStacks)
		} else if finished == nil || finished.score < force.score {
			finished = force
		}
	}
	return finished
}

func pushStackDedup(stack *Stack, newStacks *[]*Stack) {
	for i, other := range *newStacks {
		if other.pos == stack.pos && other.sameState(stack) {
			if other.score < stack.score {
				(*newStacks)[i] = stack
			}
			return
		}
	}
	*newStacks = append(*newStacks, stack)
}

func findFinished(stacks []*Stack) *Stack {
	var best *Stack
	for _, s := range stacks {
		if s.pos == s.p.stream.end && s.p.parser.stateFlag(s.state, stateFlagAccepting) && (best == nil || best.score < s.score) {
			best = s
		}
	}
	return best
}

func (ps *parse) stackToTree(s *Stack) *Tree {
	s.close()
	return buildTree(ps.parser, s.fullBuffer(), ps.reused, ps.topTerm, ps.ranges[0].From, s.pos-ps.ranges[0].From)
}

// fullBuffer is the stack's buffer with the parts it shares with the
// stacks it was split from, as @lezer/lr's StackBufferCursor reads it.
func (s *Stack) fullBuffer() []int {
	var parts [][]int
	cur, end := s, len(s.buffer)
	for {
		parts = append(parts, cur.buffer[:end])
		if cur.parent == nil {
			break
		}
		end = cur.bufferBase - cur.parent.bufferBase
		cur = cur.parent
	}
	slices.Reverse(parts)
	return slices.Concat(parts...)
}
