// Ported from @lezer/lr 1.4.11 (src/decode.ts, src/constants.ts), MIT
// licensed, Copyright (C) 2018 by Marijn Haverbeke and others. See LICENSE.

package lezer

// The action and table encodings lezer-generator writes, by their names in
// @lezer/lr's constants.ts.
const (
	actionReduceFlag  = 1 << 16
	actionValueMask   = 1<<16 - 1
	actionReduceShift = 19
	actionRepeatFlag  = 1 << 17
	actionGotoFlag    = 1 << 17
	actionStayFlag    = 1 << 18

	stateFlagSkipped   = 1
	stateFlagAccepting = 2

	slotFlags                     = 0
	slotActions                   = 1
	slotSkip                      = 2
	slotTokenizerMask             = 3
	slotDefaultReduce             = 4
	slotForcedReduce              = 5
	stateSize                     = 6
	seqEnd                        = 0xffff
	seqNext                       = 1
	seqOther                      = 2
	termErr                       = 0
	specializeExtend              = 1
	lookAheadMargin               = 25
	recoverInsert                 = 200
	recoverDelete                 = 190
	recoverReduce                 = 100
	recoverMaxNext                = 4
	recoverMaxInsertStackDepth    = 300
	recoverDampenInsertStackDepth = 120
	recoverMinBigReduction        = 2000
)

// Tunables of the parse loop, by their names in @lezer/lr's parse.ts.
const (
	recDistance              = 5
	recMaxRemainingPerStep   = 3
	recMinBufferLengthPrune  = 500
	recForceReduceLimit      = 10
	recCutDepth              = 2800 * 3
	recCutTo                 = 2000 * 3
	recMaxLeftAssocReduction = 300
	recMaxStackCount         = 12
)

// The base-46 text encoding of lezer-generator's encode.ts.
const (
	encodeBigValCode = 126
	encodeBigVal     = 0xffff
	encodeStart      = 32
	encodeGap1       = 34
	encodeGap2       = 92
	encodeBase       = 46
)

// decodeArray reads an array lezer-generator encoded as text: its first
// value is the array's length, the rest its elements.
func decodeArray(input string) []uint32 {
	var out []uint32
	first := true
	for pos := 0; pos < len(input); {
		value := uint32(0)
		for {
			next := int(input[pos])
			pos++
			if next == encodeBigValCode {
				value = encodeBigVal
				break
			}
			if next >= encodeGap2 {
				next--
			}
			if next >= encodeGap1 {
				next--
			}
			digit := uint32(next - encodeStart)
			stop := digit >= encodeBase
			if stop {
				digit -= encodeBase
			}
			value += digit
			if stop {
				break
			}
			value *= encodeBase
		}
		if first {
			out = make([]uint32, 0, value)
			first = false
			continue
		}
		out = append(out, value)
	}
	return out
}

// decode16 is decodeArray for the tables @lezer/lr keeps as Uint16Array.
func decode16(input string) []uint16 {
	wide := decodeArray(input)
	out := make([]uint16, len(wide))
	for i, v := range wide {
		out[i] = uint16(v)
	}
	return out
}
