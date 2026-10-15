// Ported from @lezer/lr 1.4.11 (src/stack.ts: Stack, StackContext,
// SimulatedStack), MIT licensed, Copyright (C) 2018 by Marijn Haverbeke and
// others. See LICENSE.

package lezer

import "slices"

// Stack is one parse stack: a parse runs several when the grammar is
// ambiguous or while it recovers from an error. Tokenizers and context
// trackers read the parse state through it.
type Stack struct {
	p *parse
	// stack holds (state, start position, buffer index) triplets for all
	// but the top state.
	stack []int
	state int
	// reducePos is where the next reduce takes place: less than pos when
	// skipped tokens were shifted that belong outside the reduction.
	reducePos int
	pos       int
	score     int
	// buffer holds (type, start, end, size) quads for the nodes made so
	// far, in post-order. A split stack shares its parent's buffer up to
	// bufferBase, an offset counted across the chain of parents.
	buffer     []int
	bufferBase int
	curContext *stackContext
	lookAhead  int
	parent     *Stack
}

type stackContext struct {
	tracker *ContextTracker
	context any
	hash    int
}

func newStackContext(t *ContextTracker, context any) *stackContext {
	c := &stackContext{tracker: t, context: context}
	if !t.NonStrict && t.Hash != nil {
		c.hash = t.Hash(context)
	}
	return c
}

func startStack(p *parse, state, pos int) *Stack {
	s := &Stack{p: p, state: state, reducePos: pos, pos: pos}
	if cx := p.parser.context; cx != nil {
		s.curContext = newStackContext(cx, cx.Start)
	}
	return s
}

// Pos is the input position the stack has parsed up to.
func (s *Stack) Pos() int { return s.pos }

// Context is the stack's context tracker value, nil without a tracker.
func (s *Stack) Context() any {
	if s.curContext == nil {
		return nil
	}
	return s.curContext.context
}

// DialectEnabled reports whether the dialect with the given index (its
// place in the grammar's dialect list) is enabled.
func (s *Stack) DialectEnabled(id int) bool { return s.p.parser.dialect.flags[id] }

func (s *Stack) pushState(state, start int) {
	s.stack = append(s.stack, s.state, start, s.bufferBase+len(s.buffer))
	s.state = state
}

func (s *Stack) reduce(action int) {
	depth, typ := action>>actionReduceShift, action&actionValueMask
	p := s.p.parser
	lookaheadRecord := s.reducePos < s.pos-lookAheadMargin && s.setLookAhead(s.pos)
	s.score += p.dynamicPrecedence(typ)
	if depth == 0 {
		s.reduceEmpty(typ, lookaheadRecord)
		return
	}
	// A StayFlag reduction also consumes the dummy parent of the skipped
	// expression and the state to stay in.
	base := len(s.stack) - (depth-1)*3
	if action&actionStayFlag != 0 {
		base -= 6
	}
	start := s.p.ranges[0].From
	if base > 0 {
		start = s.stack[base-2]
	}
	if typ < p.minRepeatTerm && start == s.reducePos && s.reducePos < s.pos {
		s.reducePos = s.pos
	}
	s.trackBigReduction(typ, start, s.reducePos-start)
	bufferBase := 0
	if base > 0 {
		bufferBase = s.stack[base-1]
	}
	count := s.bufferBase + len(s.buffer) - bufferBase
	if typ < p.minRepeatTerm || action&actionRepeatFlag != 0 {
		pos := s.reducePos
		if p.stateFlag(s.state, stateFlagSkipped) {
			pos = s.pos
		}
		s.storeNode(typ, start, pos, count+4, true)
	}
	if action&actionStayFlag != 0 {
		s.state = s.stack[base]
	} else {
		s.state = p.getGoto(s.stack[base-3], typ, true)
	}
	s.stack = s.stack[:base]
	s.reduceContext(typ, start)
}

// reduceEmpty applies a zero-depth reduction, which adds to the stack
// without popping anything off.
func (s *Stack) reduceEmpty(typ int, lookaheadRecord bool) {
	p := s.p.parser
	if typ < p.minRepeatTerm && s.reducePos < s.pos {
		s.reducePos = s.pos
	}
	s.pushState(p.getGoto(s.state, typ, true), s.reducePos)
	if typ < p.minRepeatTerm {
		size := 4
		if lookaheadRecord {
			size = 8
		}
		s.storeNode(typ, s.reducePos, s.reducePos, size, true)
	}
	s.reduceContext(typ, s.reducePos)
}

// trackBigReduction notes reductions of a size that hints at an overly
// deep left-associative tree, which the stack depth limit would not catch.
func (s *Stack) trackBigReduction(typ, start, size int) {
	types := s.p.parser.types
	if size < recoverMinBigReduction || (typ < len(types) && types[typ].Anonymous()) {
		return
	}
	p := s.p
	if start == p.lastBigReductionStart {
		p.bigReductionCount++
		p.lastBigReductionSize = size
	} else if p.lastBigReductionSize < size {
		p.bigReductionCount = 1
		p.lastBigReductionStart = start
		p.lastBigReductionSize = size
	}
}

func (s *Stack) storeNode(term, start, end, size int, mustSink bool) {
	if term == termErr && s.mergeError(start, end) {
		return
	}
	if !mustSink || s.pos == end {
		s.buffer = append(s.buffer, term, start, end, size)
		return
	}
	// Skipped nodes after end have to move forward past the new node.
	index := len(s.buffer)
	if index > 0 && (s.buffer[index-4] != termErr || s.buffer[index-1] < 0) {
		mustMove := false
		for scan := index; scan > 0 && s.buffer[scan-2] > end; scan -= 4 {
			if s.buffer[scan-1] >= 0 {
				mustMove = true
				break
			}
		}
		if mustMove {
			s.buffer = append(s.buffer, 0, 0, 0, 0)
			for index > 0 && s.buffer[index-2] > end {
				copy(s.buffer[index:index+4], s.buffer[index-4:index])
				index -= 4
				if size > 4 {
					size -= 4
				}
			}
		}
	}
	if index == len(s.buffer) {
		s.buffer = append(s.buffer, 0, 0, 0, 0)
	}
	s.buffer[index], s.buffer[index+1], s.buffer[index+2], s.buffer[index+3] = term, start, end, size
}

// mergeError drops an empty error node, or extends the error node on top
// of the buffer, instead of storing a new one next to it.
func (s *Stack) mergeError(start, end int) bool {
	if len(s.stack) > 0 && s.stack[len(s.stack)-1] >= len(s.buffer)+s.bufferBase {
		return false
	}
	top := len(s.buffer)
	if top == 0 || s.buffer[top-4] != termErr || s.buffer[top-1] <= -1 {
		return false
	}
	if start == end {
		return true
	}
	if s.buffer[top-2] >= start {
		s.buffer[top-2] = end
		return true
	}
	return false
}

func (s *Stack) shift(action, typ, start, end int) {
	p := s.p.parser
	switch {
	case action&actionGotoFlag != 0:
		s.pushState(action&actionValueMask, s.pos)
	case action&actionStayFlag == 0:
		nextState := action
		s.pos = end
		skipped := p.stateFlag(nextState, stateFlagSkipped)
		// Skipped or zero-length non-tree tokens don't move reducePos.
		if !skipped && (end > start || typ <= p.maxNode) {
			s.reducePos = end
		}
		at := min(start, s.reducePos)
		if skipped {
			at = start
		}
		s.pushState(nextState, at)
		s.shiftContext(typ, start)
		if typ <= p.maxNode {
			s.buffer = append(s.buffer, typ, start, end, 4)
		}
	default:
		// Shift-and-stay: a skipped token.
		s.pos = end
		s.shiftContext(typ, start)
		if typ <= p.maxNode {
			s.buffer = append(s.buffer, typ, start, end, 4)
		}
	}
}

func (s *Stack) apply(action, next, nextStart, nextEnd int) {
	if action&actionReduceFlag != 0 {
		s.reduce(action)
	} else {
		s.shift(action, next, nextStart, nextEnd)
	}
}

func (s *Stack) split() *Stack {
	parent := s
	off := len(parent.buffer)
	// The top error node may still merge with others, and skipped tokens
	// past reducePos may still be reordered: both go to the new buffer.
	if off > 0 && parent.buffer[off-4] == termErr {
		off -= 4
	}
	for off > 0 && parent.buffer[off-2] > parent.reducePos {
		off -= 4
	}
	buffer, base := slices.Clone(parent.buffer[off:]), parent.bufferBase+off
	for parent != nil && base == parent.bufferBase {
		parent = parent.parent
	}
	return &Stack{
		p: s.p, stack: slices.Clone(s.stack), state: s.state, reducePos: s.reducePos, pos: s.pos,
		score: s.score, buffer: buffer, bufferBase: base, curContext: s.curContext,
		lookAhead: s.lookAhead, parent: parent,
	}
}

func (s *Stack) sameState(other *Stack) bool {
	if s.state != other.state || len(s.stack) != len(other.stack) {
		return false
	}
	for i := 0; i < len(s.stack); i += 3 {
		if s.stack[i] != other.stack[i] {
			return false
		}
	}
	return true
}

// CanShift reports whether term could be shifted on the stack, after the
// reductions it would take: for an external tokenizer that should emit a
// token only where it applies.
func (s *Stack) CanShift(term int) bool {
	p := s.p.parser
	sim := simulatedStack{start: s, state: s.state, stack: s.stack, base: len(s.stack)}
	for {
		action := p.stateSlot(sim.state, slotDefaultReduce)
		if action == 0 {
			action = p.hasAction(sim.state, term)
		}
		if action == 0 {
			return false
		}
		if action&actionReduceFlag == 0 {
			return true
		}
		sim.reduce(action)
	}
}

// simulatedStack runs reductions to look ahead without changing the stack
// it starts from.
type simulatedStack struct {
	start  *Stack
	state  int
	stack  []int
	base   int
	copied bool
}

func (sim *simulatedStack) reduce(action int) {
	term, depth := action&actionValueMask, action>>actionReduceShift
	if depth == 0 {
		// The state goes on the end of the array, not at base: after a
		// deeper reduction the two differ, as they do in @lezer/lr.
		if !sim.copied {
			sim.stack, sim.copied = slices.Clone(sim.stack), true
		}
		sim.stack = append(sim.stack, sim.state, 0, 0)
		sim.base += 3
	} else {
		sim.base -= (depth - 1) * 3
	}
	sim.state = sim.start.p.parser.getGoto(sim.stack[sim.base-3], term, true)
}
