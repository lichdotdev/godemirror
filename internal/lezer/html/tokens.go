// Ported from @lezer/html 1.3.13 (src/tokens.js), MIT licensed, Copyright
// (C) 2018 by Marijn Haverbeke and others. See internal/lezer/LICENSE.

package html

import (
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/lichdotdev/godemirror/internal/lezer"
)

const (
	lessThan    = '<'
	greaterThan = '>'
	slash       = '/'
	question    = '?'
	bang        = '!'
	dash        = '-'
	newline     = '\n'
	carriage    = '\r'
	// firstNonASCIINameChar is where tokens.js starts taking every code
	// unit as a name character.
	firstNonASCIINameChar = 161
)

func set(names string) map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(names) {
		m[n] = true
	}
	return m
}

var (
	selfClosers      = set("area base br col command embed frame hr img input keygen link meta param source track wbr menuitem")
	implicitlyClosed = set("dd li optgroup option p rp rt tbody td tfoot th tr")
	closeOnOpen      = map[string]map[string]bool{
		"dd":       set("dd dt"),
		"dt":       set("dd dt"),
		"li":       set("li"),
		"option":   set("option optgroup"),
		"optgroup": set("optgroup"),
		"p": set("address article aside blockquote dir div dl fieldset footer form " +
			"h1 h2 h3 h4 h5 h6 header hgroup hr menu nav ol p pre section table ul"),
		"rp":    set("rp rt"),
		"rt":    set("rp rt"),
		"tbody": set("tbody tfoot"),
		"td":    set("td th"),
		"tfoot": set("tbody"),
		"th":    set("td th"),
		"thead": set("tbody tfoot"),
		"tr":    set("tr"),
	}
)

// inherited reports whether a lowercased name is a key every object
// literal of tokens.js has through Object.prototype, so that its lookups
// find something truthy there too.
func inherited(name string) bool { return name == "constructor" || name == "__proto__" }

func nameChar(ch int) bool {
	return ch == '-' || ch == '.' || ch == ':' || ch >= 'A' && ch <= 'Z' || ch == '_' || ch >= 'a' && ch <= 'z' || ch >= firstNonASCIINameChar
}

// tagNameAfter is the lowercased tag name offset units from the stream's
// position. ok is false for a "<?" or "<!", and the name is "" when it is
// just missing.
func tagNameAfter(in *lezer.InputStream, offset int) (name string, ok bool) {
	var units []uint16
	next := in.Peek(offset)
	for nameChar(next) {
		units = append(units, uint16(next))
		offset++
		next = in.Peek(offset)
	}
	if len(units) > 0 {
		return strings.ToLower(string(utf16.Decode(units))), true
	}
	return "", next != question && next != bang
}

// elementContext is an open element, by name, inside its parent.
type elementContext struct {
	name   string
	parent *elementContext
}

// contextValue keeps a nil context an untyped nil, so the tracker's
// values compare as @lezer/lr compares them.
func contextValue(cx *elementContext) any {
	if cx == nil {
		return nil
	}
	return cx
}

func contextOf(s *lezer.Stack) *elementContext {
	cx, _ := s.Context().(*elementContext)
	return cx
}

var startTagTerms = []int{nodeStartTag, nodeStartSelfClosingTag, nodeStartScriptTag, nodeStartStyleTag, nodeStartTextareaTag}

var elementTracker = &lezer.ContextTracker{
	Shift: func(context any, term int, _ *lezer.Stack, in *lezer.InputStream) any {
		if !slices.Contains(startTagTerms, term) {
			return context
		}
		name, _ := tagNameAfter(in, 1)
		parent, _ := context.(*elementContext)
		return &elementContext{name: name, parent: parent}
	},
	Reduce: func(context any, term int, _ *lezer.Stack, _ *lezer.InputStream) any {
		if cx, _ := context.(*elementContext); term == nodeElement && cx != nil {
			return contextValue(cx.parent)
		}
		return context
	},
	NonStrict: true,
}

func tagStart(in *lezer.InputStream, s *lezer.Stack) {
	if in.Next != lessThan {
		if in.Next < 0 && contextOf(s) != nil {
			in.AcceptToken(termMissingCloseTag, 0)
		}
		return
	}
	in.Advance(1)
	closing := in.Next == slash
	if closing {
		in.Advance(1)
	}
	name, ok := tagNameAfter(in, 0)
	switch {
	case !ok:
	case name == "" && closing:
		in.AcceptToken(nodeIncompleteCloseTag, 0)
	case name == "":
		in.AcceptToken(nodeIncompleteTag, 0)
	case closing:
		closeTag(in, s, name)
	default:
		openTag(in, s, name)
	}
}

func closeTag(in *lezer.InputStream, s *lezer.Stack, name string) {
	cx := contextOf(s)
	switch {
	case cx != nil && name == cx.name:
		in.AcceptToken(nodeStartCloseTag, 0)
	case cx != nil && cx.name != "" && (implicitlyClosed[cx.name] || inherited(cx.name)):
		in.AcceptToken(termMissingCloseTag, -2)
	case s.DialectEnabled(dialectNoMatch):
		in.AcceptToken(nodeNoMatchStartCloseTag, 0)
	default:
		for c := cx; c != nil; c = c.parent {
			if c.name == name {
				return
			}
		}
		in.AcceptToken(nodeMismatchedStartCloseTag, 0)
	}
}

func openTag(in *lezer.InputStream, s *lezer.Stack, name string) {
	switch {
	case name == "script":
		in.AcceptToken(nodeStartScriptTag, 0)
	case name == "style":
		in.AcceptToken(nodeStartStyleTag, 0)
	case name == "textarea":
		in.AcceptToken(nodeStartTextareaTag, 0)
	case selfClosers[name]:
		in.AcceptToken(nodeStartSelfClosingTag, 0)
	case closesOnOpen(contextOf(s), name):
		in.AcceptToken(termMissingCloseTag, -1)
	default:
		in.AcceptToken(nodeStartTag, 0)
	}
}

// closesOnOpen reports whether opening name implicitly closes the open
// element cx.
func closesOnOpen(cx *elementContext, name string) bool {
	if cx == nil {
		return false
	}
	closed, ok := closeOnOpen[cx.name]
	return ok && (closed[name] || inherited(name))
}

func commentContent(in *lezer.InputStream, _ *lezer.Stack) {
	const minContent = 3
	for dashes, i := 0, 0; ; i++ {
		if in.Next < 0 {
			if i > 0 {
				in.AcceptToken(termCommentContent, 0)
			}
			return
		}
		switch {
		case in.Next == dash:
			dashes++
		case in.Next == greaterThan && dashes >= 2:
			if i >= minContent {
				in.AcceptToken(termCommentContent, -2)
			}
			return
		default:
			dashes = 0
		}
		in.Advance(1)
	}
}

func inForeignElement(cx *elementContext) bool {
	for ; cx != nil; cx = cx.parent {
		if cx.name == "svg" || cx.name == "math" {
			return true
		}
	}
	return false
}

func endTag(in *lezer.InputStream, s *lezer.Stack) {
	switch {
	case in.Next == slash && in.Peek(1) == greaterThan:
		term := nodeEndTag
		if s.DialectEnabled(dialectSelfClosing) || inForeignElement(contextOf(s)) {
			term = nodeSelfClosingEndTag
		}
		in.AcceptToken(term, 2)
	case in.Next == greaterThan:
		in.AcceptToken(nodeEndTag, 1)
	}
}

// contentTokenizer reads the raw text of a script, style or textarea
// element up to its close tag, matched case-sensitively, one line at a
// time.
func contentTokenizer(tag string, textToken, endToken int) lezer.Tokenizer {
	// state counts what is matched: 1 for "<", 2 for "</", then one more
	// per letter of the tag; lastState is the whole tag.
	lastState := 2 + len(tag)
	return lezer.Tokenizer{Token: func(in *lezer.InputStream, _ *lezer.Stack) {
		for state, matchedLen, i := 0, 0, 0; ; i++ {
			if in.Next < 0 {
				if i > 0 {
					in.AcceptToken(textToken, 0)
				}
				return
			}
			switch {
			case state == 0 && in.Next == lessThan,
				state == 1 && in.Next == slash,
				state >= 2 && state < lastState && in.Next == int(tag[state-2]):
				state++
				matchedLen++
			case state == lastState && in.Next == greaterThan:
				if i > matchedLen {
					in.AcceptToken(textToken, -matchedLen)
				} else {
					in.AcceptToken(endToken, -(matchedLen - 2))
				}
				return
			case (in.Next == newline || in.Next == carriage) && i > 0:
				in.AcceptToken(textToken, 1)
				return
			default:
				state, matchedLen = 0, 0
			}
			in.Advance(1)
		}
	}}
}

var externals = lezer.Externals{
	Tokenizers: map[string]lezer.Tokenizer{
		"scriptTokens":   contentTokenizer("script", termScriptText, nodeStartCloseScriptTag),
		"styleTokens":    contentTokenizer("style", termStyleText, nodeStartCloseStyleTag),
		"textareaTokens": contentTokenizer("textarea", termTextareaText, nodeStartCloseTextareaTag),
		"endTag":         {Token: endTag},
		"tagStart":       {Token: tagStart, Contextual: true},
		"commentContent": {Token: commentContent},
	},
	Context: elementTracker,
}
