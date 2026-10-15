// Ported from @codemirror/language 6.13.1 (src/stream-parser.ts: the Parse
// of a StreamLanguage, readToken, TokenTable and createTokenType), MIT
// licensed, Copyright (C) 2018-2021 by Marijn Haverbeke and others. See
// LICENSE.

// Package stream highlights a document with a CodeMirror 5-style stream
// mode, as CodeMirror's StreamLanguage does for the languages
// @codemirror/legacy-modes covers and Lezer does not.
package stream

import (
	"strings"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/tags"
)

// Mode is a stream mode with its state: a fresh one for each document,
// since a mode carries its state from line to line.
type Mode interface {
	// Token reads one token from s and returns its style, "" for none: a
	// legacy name ("variable", "def") or tag names joined by "." and " ",
	// as @codemirror/language's legacy token table reads them.
	Token(s *Stream) string
}

const (
	// maxLineLength is stream-parser.ts's C.MaxLineLength: a line is not
	// tokenized past the first token starting after it.
	maxLineLength = 10000
	// maxTokenAttempts is how many times readToken asks a mode for a token
	// before giving up on one that does not move.
	maxTokenAttempts = 10
)

// Highlight calls span, in order, for every token of text (UTF-16 code
// units, split into lines at "\n" alone) whose style names tags, as
// highlightTree does with a highlighter that gives every tag its own class
// over the tree a StreamLanguage parses. Positions are code units.
//
// A mode that returns a token without reading anything ten times in a row
// is a bug, and Highlight panics, where CodeMirror's parse throws.
func Highlight(text []uint16, mode Mode, sets *lezer.TagSets, span func(from, to int, set *lezer.TagSet)) {
	e := &emitter{sets: sets, span: span, types: map[string]tokenType{}}
	for lineStart := 0; lineStart <= len(text); {
		end := lineStart
		for end < len(text) && text[end] != '\n' {
			end++
		}
		s := &Stream{line: text[lineStart:end]}
		for !s.EOL() {
			if style := readToken(mode, s); style != "" {
				e.emit(e.resolve(style), lineStart+s.start, lineStart+s.pos)
			}
			if s.start > maxLineLength {
				break
			}
		}
		lineStart = end + 1
	}
	e.flush()
}

func readToken(mode Mode, s *Stream) string {
	s.start = s.pos
	for range maxTokenAttempts {
		style := mode.Token(s)
		if s.pos > s.start {
			return style
		}
	}
	panic("stream: mode failed to advance the stream")
}

// tokenType is the node type a StreamLanguage makes for a style: the same
// key, the same type, whatever style named it.
type tokenType struct {
	key string
	set *lezer.TagSet
}

// emitter merges each token with the one before it when they touch and
// have the same type, as the parse does with mergeTokens, and passes on
// what has tags. highlightTree never merges two tokens itself.
type emitter struct {
	sets     *lezer.TagSets
	span     func(from, to int, set *lezer.TagSet)
	types    map[string]tokenType
	last     tokenType
	from, to int
	pending  bool
}

func (e *emitter) emit(t tokenType, from, to int) {
	if e.pending && t.key == e.last.key && from == e.to {
		e.to = to
		return
	}
	e.flush()
	e.last, e.from, e.to, e.pending = t, from, to, true
}

func (e *emitter) flush() {
	if e.pending && e.last.set != nil {
		e.span(e.from, e.to, e.last.set)
	}
}

// legacyNames is stream-parser.ts's defaultTable: the CodeMirror 5 style
// names that are not tag names.
var legacyNames = map[string]string{
	"variable":   "variableName",
	"variable-2": "variableName.special",
	"string-2":   "string.special",
	"def":        "variableName.definition",
	"tag":        "tagName",
	"attribute":  "attributeName",
	"type":       "typeName",
	"builtin":    "variableName.standard",
	"qualifier":  "modifier",
	"error":      "invalid",
	"header":     "heading",
	"property":   "propertyName",
}

// resolve is the type of a style, as TokenTable.resolve gives it.
func (e *emitter) resolve(style string) tokenType {
	if t, ok := e.types[style]; ok {
		return t
	}
	tagStr := style
	if name, ok := legacyNames[style]; ok {
		tagStr = name
	}
	t := e.typeOf(tagStr)
	e.types[style] = t
	return t
}

// typeOf is createTokenType: every space-separated name of tagStr is a tag
// followed by the modifiers to apply to it, joined by "."; a part that is
// neither (tags.Parse's own syntax included), a modifier with no tag before
// it and a tag after the first are skipped, where CodeMirror warns about
// them. A style naming no tag at all
// is the untagged type.
func (e *emitter) typeOf(tagStr string) tokenType {
	var found []*tags.Tag
	for name := range strings.SplitSeq(tagStr, " ") {
		expr := ""
		for part := range strings.SplitSeq(name, ".") {
			if strings.ContainsAny(part, "()") {
				continue
			}
			if _, err := tags.Parse(part); err == nil {
				if expr == "" {
					expr = part
				}
			} else if isModifier(part) && expr != "" {
				expr = part + "(" + expr + ")"
			}
		}
		if expr != "" {
			tag, err := tags.Parse(expr)
			if err != nil {
				panic("stream: " + err.Error())
			}
			found = append(found, tag)
		}
	}
	if len(found) == 0 {
		return tokenType{}
	}
	set := e.sets.Of(found...)
	key := strings.ReplaceAll(tagStr, " ", "_")
	for _, tag := range set.Tags {
		key += " " + tag.String()
	}
	return tokenType{key: key, set: set}
}

func isModifier(name string) bool {
	_, err := tags.Parse(name + "(name)")
	return err == nil
}
