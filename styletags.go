package godemirror

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/tags"
)

// StyleTags adds styleTags rules to one language's Lezer grammar, as a
// CodeMirror caller does with language.configure({props: [styleTags({...})]}):
// a node the rules match takes their tags, and a rule wins over the
// grammar's own when its path is at least as deep. It is how the tags a
// grammar gives can be made finer, telling apart what the tree tells apart
// and the grammar's own rules do not, as RicherJavaTags does.
type StyleTags struct {
	// Language names the grammar by its @lezer package: "css", "go",
	// "html", "java", "javascript" (JavaScript, JSX, TypeScript and TSX
	// alike), "python" or "rust". Kotlin and shell have no Lezer grammar,
	// so no rules.
	Language string
	// Rules are styleTags' entries, in the order the object would list
	// them.
	Rules []StyleRule
}

// StyleRule is one styleTags entry: space-separated paths and the tags the
// nodes they name take. A path is a node name or group, optionally under
// its parents ("ArgumentList/Identifier", "*" for any parent), ending in
// "/..." to give the tags to everything inside the node too, or in "!" to
// stop matching inside it. A name with "/" or "!" in it is quoted, as in
// JSON.
type StyleRule struct {
	Selector string
	Tags     []*tags.Tag
}

// RicherJavaTags tags Java finer than @lezer/java does, where its tree
// tells more than its styleTags do: an enum constant is
// constant(variableName), an annotation's "@" and name annotation, a field
// definition(propertyName), and a package or import path namespace. A field
// read, as in a.b, stays variableName: a path cannot tell its object from
// its field, both being an Identifier under FieldAccess.
func RicherJavaTags() StyleTags {
	return StyleTags{Language: "java", Rules: []StyleRule{
		{Selector: "EnumConstant/Definition", Tags: []*tags.Tag{tags.Constant(tags.VariableName)}},
		{Selector: "MarkerAnnotation Annotation MarkerAnnotation/Identifier Annotation/Identifier", Tags: []*tags.Tag{tags.Annotation}},
		{Selector: "MarkerAnnotation/ScopedIdentifier! Annotation/ScopedIdentifier!", Tags: []*tags.Tag{tags.Annotation}},
		{Selector: "FieldDeclaration/VariableDeclarator/Definition", Tags: []*tags.Tag{tags.Definition(tags.PropertyName)}},
		{Selector: "PackageDeclaration/Identifier ImportDeclaration/Identifier", Tags: []*tags.Tag{tags.Namespace}},
		{Selector: "PackageDeclaration/ScopedIdentifier! ImportDeclaration/ScopedIdentifier!", Tags: []*tags.Tag{tags.Namespace}},
	}}
}

// styledParsers are the parsers styles gave grammars, by grammar and
// rules, so that one set of rules keeps one parser and its trees go on
// being reparsed incrementally.
var styledParsers sync.Map

// lezerParser is the parser for filename, with the rules styles add to its
// grammar, nil for a file no Lezer grammar claims. A style naming no Lezer
// grammar, or a rule whose path does not parse, is an error.
func lezerParser(filename string, styles []StyleTags) (*lezer.Parser, error) {
	for _, s := range styles {
		if !isLezerLanguage(s.Language) {
			return nil, fmt.Errorf("StyleTags for %q: want a Lezer grammar, one of %s", s.Language, strings.Join(lezerLanguageNames(), ", "))
		}
	}
	lang, ok := lezerLanguages[strings.ToLower(filepath.Ext(filename))]
	if !ok {
		return nil, nil
	}
	var mine []StyleTags
	for _, s := range styles {
		if s.Language == lang.name {
			mine = append(mine, s)
		}
	}
	base := lang.parser()
	if len(mine) == 0 {
		return base, nil
	}
	key := styledKey{base, rulesKey(mine)}
	if p, ok := styledParsers.Load(key); ok {
		return p.(*lezer.Parser), nil
	}
	p := base
	for _, s := range mine {
		byName, err := lezer.StyleTags(specsOf(s.Rules))
		if err != nil {
			return nil, fmt.Errorf("StyleTags for %q: %w", s.Language, err)
		}
		p = p.WithStyleTags(byName)
	}
	stored, _ := styledParsers.LoadOrStore(key, p)
	return stored.(*lezer.Parser), nil
}

type styledKey struct {
	base  *lezer.Parser
	rules string
}

// rulesKey writes styles' rules down whole, as the key of the parser they
// make.
func rulesKey(styles []StyleTags) string {
	var b strings.Builder
	for _, s := range styles {
		for _, r := range s.Rules {
			fmt.Fprintf(&b, "%q", r.Selector)
			for _, t := range r.Tags {
				fmt.Fprintf(&b, " %s", t)
			}
			b.WriteByte('\n')
		}
		b.WriteByte(0)
	}
	return b.String()
}

func specsOf(rules []StyleRule) []lezer.StyleSpec {
	specs := make([]lezer.StyleSpec, len(rules))
	for i, r := range rules {
		specs[i] = lezer.StyleSpec{Selector: r.Selector, Tags: r.Tags}
	}
	return specs
}

func isLezerLanguage(name string) bool {
	return slices.Contains(lezerLanguageNames(), name)
}

func lezerLanguageNames() []string {
	names := map[string]bool{}
	for _, l := range lezerLanguages {
		names[l.name] = true
	}
	return slices.Sorted(maps.Keys(names))
}
