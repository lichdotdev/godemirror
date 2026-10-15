// Ported from @lezer/lr 1.4.11 (src/token.ts: TokenGroup, LocalTokenGroup,
// ExternalTokenizer, readToken; src/parse.ts: TokenCache), MIT licensed,
// Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

// Tokenizer reads a token at the stream's position and accepts it with
// AcceptToken, as @lezer/lr's ExternalTokenizer does. Contextual tokens are
// re-read in every parse state instead of cached by position; a Fallback
// tokenizer runs even after an earlier one produced a token; an Extend
// tokenizer's token is tried alongside the next tokenizer's.
type Tokenizer struct {
	Token                        func(in *InputStream, s *Stack)
	Contextual, Fallback, Extend bool
}

// tokenGroup reads the tokens of group id from the grammar's token data.
func tokenGroup(data []uint16, id int) Tokenizer {
	return Tokenizer{Token: func(in *InputStream, s *Stack) {
		p := s.p.parser
		readToken(data, in, s, id, p.data, p.tokenPrec)
	}}
}

// localTokenGroup reads the tokens of a @local tokens block, its else
// token covering whatever it skips.
func localTokenGroup(spec LocalGroupSpec) Tokenizer {
	data := decode16(spec.Data)
	return Tokenizer{Token: func(in *InputStream, s *Stack) {
		start, skipped := in.Pos, 0
		for {
			atEOF := in.Next < 0
			nextPos, ok := in.resolveOffset(1, 1)
			readToken(data, in, s, 0, data, spec.PrecTable)
			if in.token.value > -1 {
				break
			}
			if spec.ElseToken < 0 {
				return
			}
			if !atEOF {
				skipped++
			}
			if !ok {
				break
			}
			in.reset(nextPos, in.token)
		}
		if skipped > 0 {
			in.reset(start, in.token)
			in.AcceptToken(spec.ElseToken, skipped)
		}
	}}
}

// readToken runs the tokenizer automaton in data from the stream's
// position, as long as states that can still reach a token of group are
// reachable, accepting the best token met on the way.
func readToken(data []uint16, in *InputStream, s *Stack, group int, precTable []uint16, precOffset int) {
	state, groupMask, d := 0, 1<<group, s.p.parser.dialect
scan:
	for {
		if groupMask&int(data[state]) == 0 {
			break
		}
		accEnd := int(data[state+1])
		acceptAt(data, in, d, state, accEnd, groupMask, precTable, precOffset)
		next, low, high := in.Next, 0, int(data[state+2])
		if in.Next < 0 && high > low && data[accEnd+high*3-3] == seqEnd {
			state = int(data[accEnd+high*3-1])
			continue
		}
		for low < high {
			mid := (low + high) >> 1
			index := accEnd + mid*3
			from, to := int(data[index]), int(data[index+1])
			if to == 0 {
				to = 0x10000
			}
			switch {
			case next < from:
				high = mid
			case next >= to:
				low = mid + 1
			default:
				state = int(data[index+2])
				in.Advance(1)
				continue scan
			}
		}
		break
	}
}

// acceptAt accepts the first token state accepts in group that the
// dialect allows and that beats the token already accepted.
func acceptAt(data []uint16, in *InputStream, d dialect, state, accEnd, groupMask int, precTable []uint16, precOffset int) {
	for i := state + 3; i < accEnd; i += 2 {
		if int(data[i+1])&groupMask == 0 {
			continue
		}
		term, cur := int(data[i]), in.token.value
		if d.allows(term) && (cur == -1 || cur == term || overrides(term, cur, precTable, precOffset)) {
			in.AcceptToken(term, 0)
			return
		}
	}
}

func findOffset(data []uint16, start, term int) int {
	for i := start; data[i] != seqEnd; i++ {
		if int(data[i]) == term {
			return i - start
		}
	}
	return -1
}

func overrides(token, prev int, table []uint16, offset int) bool {
	iPrev := findOffset(table, offset, prev)
	return iPrev < 0 || findOffset(table, offset, token) < iPrev
}

// tokenCache keeps the last token each tokenizer read, and the actions
// the current token allows.
type tokenCache struct {
	stream    *InputStream
	tokens    []*cachedToken
	mainToken *cachedToken
	actions   []int
}

func newTokenCache(p *Parser, stream *InputStream) *tokenCache {
	c := &tokenCache{stream: stream}
	for range p.tokenizers {
		c.tokens = append(c.tokens, newCachedToken())
	}
	return c
}

func (c *tokenCache) getActions(s *Stack) []int {
	actionIndex := 0
	var main *cachedToken
	p := s.p.parser
	mask := p.stateSlot(s.state, slotTokenizerMask)
	context, lookAhead := 0, 0
	if s.curContext != nil {
		context = s.curContext.hash
	}
	for i, tokenizer := range p.tokenizers {
		if (1<<i)&mask == 0 || (main != nil && !tokenizer.Fallback) {
			continue
		}
		token := c.tokens[i]
		if tokenizer.Contextual || token.start != s.pos || token.mask != mask || token.context != context {
			c.updateCachedToken(token, tokenizer, s)
			token.mask, token.context = mask, context
		}
		if token.lookAhead > token.end+lookAheadMargin {
			lookAhead = max(token.lookAhead, lookAhead)
		}
		if token.value == termErr {
			continue
		}
		startIndex := actionIndex
		if token.extended > -1 {
			actionIndex = c.addActions(s, token.extended, token.end, actionIndex)
		}
		actionIndex = c.addActions(s, token.value, token.end, actionIndex)
		if !tokenizer.Extend {
			main = token
			if actionIndex > startIndex {
				break
			}
		}
	}
	c.actions = c.actions[:actionIndex]
	if lookAhead != 0 {
		s.setLookAhead(lookAhead)
	}
	if main == nil && s.pos == c.stream.end {
		main = newCachedToken()
		main.value = p.eofTerm()
		main.start, main.end = s.pos, s.pos
		actionIndex = c.addActions(s, main.value, main.end, actionIndex)
	}
	c.mainToken = main
	return c.actions[:actionIndex]
}

func (c *tokenCache) getMainToken(s *Stack) *cachedToken {
	if c.mainToken != nil {
		return c.mainToken
	}
	main := newCachedToken()
	main.start = s.pos
	main.end = min(s.pos+1, s.p.stream.end)
	main.value = termErr
	if s.pos == s.p.stream.end {
		main.value = s.p.parser.eofTerm()
	}
	return main
}

func (c *tokenCache) updateCachedToken(token *cachedToken, tokenizer Tokenizer, s *Stack) {
	start := c.stream.clipPos(s.pos)
	tokenizer.Token(c.stream.reset(start, token), s)
	if token.value <= -1 {
		token.value = termErr
		token.end = c.stream.clipPos(start + 1)
		return
	}
	p := s.p.parser
	for i, term := range p.specialized {
		if term != token.value {
			continue
		}
		result := p.specializers[i](c.stream.read(token.start, token.end), s)
		if result >= 0 && p.dialect.allows(result>>1) {
			if result&1 == 0 {
				token.value = result >> 1
			} else {
				token.extended = result >> 1
			}
			break
		}
	}
}

func (c *tokenCache) putAction(action, token, end, index int) int {
	for i := 0; i < index; i += 3 {
		if c.actions[i] == action {
			return index
		}
	}
	c.actions = append(c.actions[:index], action, token, end)
	return index + 3
}

func (c *tokenCache) addActions(s *Stack, token, end, index int) int {
	state, p := s.state, s.p.parser
	data := p.data
	for set := range 2 {
		slot := slotActions
		if set == 1 {
			slot = slotSkip
		}
		for i := p.stateSlot(state, slot); ; i += 3 {
			if data[i] == seqEnd {
				if data[i+1] == seqNext {
					i = pair(data, i+2)
				} else {
					if index == 0 && data[i+1] == seqOther {
						index = c.putAction(pair(data, i+2), token, end, index)
					}
					break
				}
			}
			if int(data[i]) == token {
				index = c.putAction(pair(data, i+1), token, end, index)
			}
		}
	}
	return index
}
