// Ported from @lezer/lr 1.4.11 (src/stack.ts: the recovery and context
// methods of Stack; src/parse.ts: ContextTracker), MIT licensed, Copyright
// (C) 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

// ContextTracker keeps a value along a parse that external tokenizers can
// read, such as the indentation in Python's grammar, the way @lezer/lr's
// ContextTracker does. Values are compared with ==, so they must be
// comparable; like @lezer/lr's they should be immutable, replaced rather
// than changed. Shift and Reduce default to keeping the value. Unless
// NonStrict, Hash tells values apart for the token cache and the tree.
type ContextTracker struct {
	Start     any
	Shift     func(context any, term int, s *Stack, in *InputStream) any
	Reduce    func(context any, term int, s *Stack, in *InputStream) any
	Hash      func(context any) int
	NonStrict bool
}

func (s *Stack) shiftContext(term, start int) {
	if s.curContext != nil && s.curContext.tracker.Shift != nil {
		s.updateContext(s.curContext.tracker.Shift(s.curContext.context, term, s, s.p.stream.reset(start, nil)))
	}
}

func (s *Stack) reduceContext(term, start int) {
	if s.curContext != nil && s.curContext.tracker.Reduce != nil {
		s.updateContext(s.curContext.tracker.Reduce(s.curContext.context, term, s, s.p.stream.reset(start, nil)))
	}
}

// Buffer records that are not nodes: a context hash change and a
// lookahead distance.
const (
	recordContextChange = -3
	recordLookAhead     = -4
)

func (s *Stack) emitContext() {
	last := len(s.buffer) - 1
	if last < 0 || s.buffer[last] != recordContextChange {
		s.storeNode(s.curContext.hash, s.reducePos, s.reducePos, recordContextChange, true)
	}
}

func (s *Stack) emitLookAhead() {
	last := len(s.buffer) - 1
	if last < 0 || s.buffer[last] != recordLookAhead {
		s.buffer = append(s.buffer, s.lookAhead, s.pos, s.pos, recordLookAhead)
	}
}

func (s *Stack) updateContext(context any) {
	if context == s.curContext.context {
		return
	}
	next := newStackContext(s.curContext.tracker, context)
	if next.hash != s.curContext.hash {
		s.emitContext()
	}
	s.curContext = next
}

func (s *Stack) setLookAhead(lookAhead int) bool {
	if lookAhead <= s.lookAhead {
		return false
	}
	s.emitLookAhead()
	s.lookAhead = lookAhead
	return true
}

func (s *Stack) close() {
	if s.curContext != nil && !s.curContext.tracker.NonStrict {
		s.emitContext()
	}
	if s.lookAhead > 0 {
		s.emitLookAhead()
	}
}

// recoverByDelete recovers from an error by skipping one token.
func (s *Stack) recoverByDelete(next, nextEnd int) {
	isNode := next <= s.p.parser.maxNode
	size := 4
	if isNode {
		s.storeNode(next, s.pos, nextEnd, 4, false)
		size = 8
	}
	s.storeNode(termErr, s.pos, nextEnd, size, false)
	s.pos, s.reducePos = nextEnd, nextEnd
	s.score -= recoverDelete
}

// recoverByInsert splits off up to recoverMaxNext stacks that each
// pretend a missing token or rule was there.
func (s *Stack) recoverByInsert(next int) []*Stack {
	if len(s.stack) >= recoverMaxInsertStackDepth {
		return nil
	}
	p := s.p.parser
	nextStates := p.nextStates(s.state)
	if len(nextStates) > recoverMaxNext<<1 || len(s.stack) >= recoverDampenInsertStackDepth {
		nextStates = s.bestNextStates(nextStates, next)
	}
	var result []*Stack
	for i := 0; i < len(nextStates) && len(result) < recoverMaxNext; i += 2 {
		st := nextStates[i+1]
		if st == s.state {
			continue
		}
		stack := s.split()
		stack.pushState(st, s.pos)
		stack.storeNode(termErr, stack.pos, stack.pos, 4, true)
		stack.shiftContext(nextStates[i], s.pos)
		stack.reducePos = s.pos
		stack.score -= recoverInsert
		result = append(result, stack)
	}
	return result
}

// bestNextStates narrows nextStates to the ones that can take next, then,
// on a shallow stack, tops them up with others.
func (s *Stack) bestNextStates(nextStates []int, next int) []int {
	p := s.p.parser
	var best []int
	for i := 0; i < len(nextStates); i += 2 {
		if st := nextStates[i+1]; st != s.state && p.hasAction(st, next) != 0 {
			best = append(best, nextStates[i], st)
		}
	}
	if len(s.stack) >= recoverDampenInsertStackDepth {
		return best
	}
	for i := 0; len(best) < recoverMaxNext<<1 && i < len(nextStates); i += 2 {
		st := nextStates[i+1]
		seen := false
		for j := 1; j < len(best); j += 2 {
			seen = seen || best[j] == st
		}
		if !seen {
			best = append(best, nextStates[i], st)
		}
	}
	return best
}

// forceReduce applies the state's forced reduction, reporting false when
// there is none that can be done.
func (s *Stack) forceReduce() bool {
	p := s.p.parser
	reduce := p.stateSlot(s.state, slotForcedReduce)
	if reduce&actionReduceFlag == 0 {
		return false
	}
	if !p.validAction(s.state, reduce) {
		depth, term := reduce>>actionReduceShift, reduce&actionValueMask
		target := len(s.stack) - depth*3
		if target < 0 || p.getGoto(s.stack[target], term, false) < 0 {
			backup, ok := s.findForcedReduction()
			if !ok {
				return false
			}
			reduce = backup
		}
		s.storeNode(termErr, s.pos, s.pos, 4, true)
		s.score -= recoverReduce
	}
	s.reducePos = s.pos
	s.reduce(reduce)
	return true
}

// findForcedReduction scans the automaton for some reduction that applies,
// for when the state's own forced reduction is not a valid action.
func (s *Stack) findForcedReduction() (int, bool) {
	p := s.p.parser
	var seen []int
	var explore func(state, depth int) (int, bool)
	explore = func(state, depth int) (int, bool) {
		for _, st := range seen {
			if st == state {
				return 0, false
			}
		}
		seen = append(seen, state)
		return p.allActions(state, func(action int) (int, bool) {
			switch {
			case action&(actionStayFlag|actionGotoFlag) != 0:
				return 0, false
			case action&actionReduceFlag != 0:
				return s.reductionAt(action, depth)
			}
			return explore(action, depth+1)
		})
	}
	return explore(s.state, 0)
}

// reductionAt is action as a reduction of the stack, found depth states
// past its top, when the stack is deep enough for it to apply.
func (s *Stack) reductionAt(action, depth int) (int, bool) {
	rDepth := (action >> actionReduceShift) - depth
	if rDepth <= 1 {
		return 0, false
	}
	term, target := action&actionValueMask, len(s.stack)-rDepth*3
	if target >= 0 && s.p.parser.getGoto(s.stack[target], term, false) >= 0 {
		return rDepth<<actionReduceShift | actionReduceFlag | term, true
	}
	return 0, false
}

func (s *Stack) forceAll() *Stack {
	for !s.p.parser.stateFlag(s.state, stateFlagAccepting) {
		if !s.forceReduce() {
			s.storeNode(termErr, s.pos, s.pos, 4, true)
			break
		}
	}
	return s
}

// deadEnd reports whether the state, directly above the bottom one, has no
// further actions.
func (s *Stack) deadEnd() bool {
	if len(s.stack) != 3 {
		return false
	}
	p := s.p.parser
	return p.data[p.stateSlot(s.state, slotActions)] == seqEnd && p.stateSlot(s.state, slotDefaultReduce) == 0
}

// restart puts the stack back in its start state. Only safe when
// deadEnd holds.
func (s *Stack) restart() {
	s.storeNode(termErr, s.pos, s.pos, 4, true)
	s.state = s.stack[0]
	s.stack = s.stack[:0]
}
