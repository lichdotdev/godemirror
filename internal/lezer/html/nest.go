// Ported from @lezer/html 1.3.13 (src/content.js) and the default nesting
// of @codemirror/lang-html 6.4.12 (src/html.ts: defaultNesting,
// defaultAttrs; src/complete.ts: eventAttributes), MIT licensed, Copyright
// (C) 2018 by Marijn Haverbeke and others. See internal/lezer/LICENSE.

package html

import (
	"regexp"
	"strings"
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/css"
	"github.com/lichdotdev/godemirror/internal/lezer/javascript"
)

// nestedTag is a parser for the content of a script or style element whose
// attributes match.
type nestedTag struct {
	attrs  func(attrs map[string]string) bool
	parser func() *lezer.Parser
}

// notLineTerminator is JavaScript's "." outside the u and s flags.
const notLineTerminator = `[^\n\r\x{2028}\x{2029}]`

var (
	jsonType       = regexp.MustCompile(`(?i)^(importmap|speculationrules|application/(` + notLineTerminator + `+\+)?json)$`)
	javaScriptType = regexp.MustCompile(`(?i)^(?:text|application)/(?:x-)?(?:java|ecma)script$|^module$|^$`)
	cssType        = regexp.MustCompile(`(?i)^(text/)?(x-)?(stylesheet|css)$`)
)

var json = sync.OnceValue(func() *lezer.Parser { return configure(javascript.Parser(), "SingleExpression") })

var styleAttribute = sync.OnceValue(func() *lezer.Parser { return configure(css.Parser(), "Styles") })

func configure(p *lezer.Parser, top string) *lezer.Parser {
	configured, err := p.Configure(lezer.Config{Top: top})
	if err != nil {
		panic("html: " + err.Error())
	}
	return configured
}

var scriptTags = []nestedTag{
	{func(a map[string]string) bool { return a["type"] == "text/typescript" || a["lang"] == "ts" }, javascript.TypeScript},
	{func(a map[string]string) bool { return a["type"] == "text/babel" || a["type"] == "text/jsx" }, javascript.JSX},
	{func(a map[string]string) bool { return a["type"] == "text/typescript-jsx" }, javascript.TSX},
	{func(a map[string]string) bool { return jsonType.MatchString(a["type"]) }, json},
	{func(a map[string]string) bool { return a["type"] == "" || javaScriptType.MatchString(a["type"]) }, javascript.Parser},
}

var styleTags = []nestedTag{
	{func(a map[string]string) bool {
		return (a["lang"] == "" || a["lang"] == "css") && (a["type"] == "" || cssType.MatchString(a["type"]))
	}, css.Parser},
}

// attributeParsers are the parsers of the attributes whose value is code.
var attributeParsers = func() map[string]func() *lezer.Parser {
	m := map[string]func() *lezer.Parser{"style": styleAttribute}
	events := "beforeunload copy cut dragstart dragover dragleave dragenter dragend " +
		"drag paste focus blur change click load mousedown mouseenter mouseleave " +
		"mouseup keydown keyup resize scroll unload"
	for _, event := range strings.Fields(events) {
		m["on"+event] = javascript.Parser
	}
	return m
}()

func nest(n *lezer.Node, in lezer.Input) *lezer.NestedParse {
	switch n.Type().ID {
	case termScriptTextNode:
		return nestContent(n, in, scriptTags)
	case termStyleTextNode:
		return nestContent(n, in, styleTags)
	case termAttribute:
		return nestAttribute(n, in)
	}
	return nil
}

// nestContent parses the text of a script or style element with the
// parser of the first tag whose attributes match its open tag's.
func nestContent(n *lezer.Node, in lezer.Input, tags []nestedTag) *lezer.NestedParse {
	attrs := attributes(n.Parent().FirstChild(), in)
	for _, tag := range tags {
		if tag.attrs(attrs) {
			return &lezer.NestedParse{Parser: tag.parser()}
		}
	}
	return nil
}

// attributes are an open tag's attribute values by name, "" for one
// without a value.
func attributes(openTag *lezer.Node, in lezer.Input) map[string]string {
	attrs := map[string]string{}
	for _, att := range children(openTag, termAttribute) {
		name := child(att, termAttributeName)
		if name == nil {
			continue
		}
		value := ""
		if v := child(att, termAttributeValue); v != nil {
			value = in.Read(v.From()+1, v.To()-1)
		} else if v := child(att, termUnquotedAttributeValue); v != nil {
			value = in.Read(v.From(), v.To())
		}
		attrs[in.Read(name.From(), name.To())] = value
	}
	return attrs
}

// nestAttribute parses the value of an attribute whose value is code,
// inside its quotes.
func nestAttribute(n *lezer.Node, in lezer.Input) *lezer.NestedParse {
	name := n.FirstChild()
	if name == nil {
		return nil
	}
	parser, ok := attributeParsers[in.Read(name.From(), name.To())]
	if !ok {
		return nil
	}
	value := n.LastChild()
	switch value.Type().ID {
	case termAttributeValue:
		// content.js means to keep the last character of a value whose
		// closing quote is missing, but tests last.isError, which a
		// SyntaxNode does not have, so it always drops it.
		from, to := value.From()+1, value.To()-1
		if to > from {
			return &lezer.NestedParse{Parser: parser(), Overlay: []lezer.Range{{From: from, To: to}}}
		}
	case termUnquotedAttributeValue:
		return &lezer.NestedParse{Parser: parser(), Overlay: []lezer.Range{{From: value.From(), To: value.To()}}}
	}
	return nil
}

// children are n's children of type id, as SyntaxNode.getChildren finds
// them.
func children(n *lezer.Node, id int) []*lezer.Node {
	var found []*lezer.Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Type().ID == id {
			found = append(found, c)
		}
	}
	return found
}

func child(n *lezer.Node, id int) *lezer.Node {
	if found := children(n, id); len(found) > 0 {
		return found[0]
	}
	return nil
}
