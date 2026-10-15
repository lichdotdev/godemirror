package godemirror

import (
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"

	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/tags"
)

// tagOfType is the tag each chroma token type stands for, picked as the tag
// CodeMirror's lezer grammars give the same construct. A type with no entry
// takes its subcategory's, then its category's; one with none is plain.
var tagOfType = map[chroma.TokenType]*tags.Tag{
	chroma.Keyword:            tags.Keyword,
	chroma.KeywordDeclaration: tags.DefinitionKeyword,
	chroma.KeywordNamespace:   tags.ModuleKeyword,
	chroma.KeywordType:        tags.TypeName,

	chroma.Name:                     tags.VariableName,
	chroma.NameAttribute:            tags.AttributeName,
	chroma.NameClass:                tags.ClassName,
	chroma.NameConstant:             tags.Constant(tags.VariableName),
	chroma.NameDecorator:            tags.Annotation,
	chroma.NameEntity:               tags.Character,
	chroma.NameException:            tags.ClassName,
	chroma.NameKeyword:              tags.Atom,
	chroma.NameLabel:                tags.LabelName,
	chroma.NameNamespace:            tags.Namespace,
	chroma.NameOperator:             tags.Operator,
	chroma.NameProperty:             tags.PropertyName,
	chroma.NameTag:                  tags.TagName,
	chroma.NameBuiltin:              tags.Standard(tags.VariableName),
	chroma.NameBuiltinPseudo:        tags.Self,
	chroma.NameVariableClass:        tags.Special(tags.VariableName),
	chroma.NameVariableGlobal:       tags.Special(tags.VariableName),
	chroma.NameVariableInstance:     tags.Special(tags.VariableName),
	chroma.NameVariableMagic:        tags.Standard(tags.VariableName),
	chroma.NameFunction:             tags.Function(tags.VariableName),
	chroma.Literal:                  tags.Literal,
	chroma.LiteralString:            tags.String,
	chroma.LiteralStringAtom:        tags.Atom,
	chroma.LiteralStringBacktick:    tags.Special(tags.String),
	chroma.LiteralStringBoolean:     tags.Bool,
	chroma.LiteralStringChar:        tags.Character,
	chroma.LiteralStringDoc:         tags.DocString,
	chroma.LiteralStringEscape:      tags.Escape,
	chroma.LiteralStringInterpol:    tags.Special(tags.String),
	chroma.LiteralStringRegex:       tags.Regexp,
	chroma.LiteralStringSymbol:      tags.Atom,
	chroma.LiteralNumber:            tags.Number,
	chroma.LiteralNumberBin:         tags.Integer,
	chroma.LiteralNumberByte:        tags.Integer,
	chroma.LiteralNumberFloat:       tags.Float,
	chroma.LiteralNumberHex:         tags.Integer,
	chroma.LiteralNumberInteger:     tags.Integer,
	chroma.LiteralNumberIntegerLong: tags.Integer,
	chroma.LiteralNumberOct:         tags.Integer,

	chroma.Operator:     tags.Operator,
	chroma.OperatorWord: tags.OperatorKeyword,

	chroma.Comment:            tags.Comment,
	chroma.CommentHashbang:    tags.Meta,
	chroma.CommentMultiline:   tags.BlockComment,
	chroma.CommentSingle:      tags.LineComment,
	chroma.CommentSpecial:     tags.DocComment,
	chroma.CommentPreproc:     tags.ProcessingInstruction,
	chroma.CommentPreprocFile: tags.String,

	chroma.GenericDeleted:    tags.Deleted,
	chroma.GenericEmph:       tags.Emphasis,
	chroma.GenericError:      tags.Invalid,
	chroma.GenericHeading:    tags.Heading,
	chroma.GenericInserted:   tags.Inserted,
	chroma.GenericStrong:     tags.Strong,
	chroma.GenericSubheading: tags.Heading,

	chroma.Error: tags.Invalid,
}

// tagOf is the tag of a token of type t. KeywordConstant tells its words
// apart by text, since lezer tags true and false bool and nil and null null.
func tagOf(t chroma.TokenType, text string) *tags.Tag {
	if t == chroma.KeywordConstant {
		return constantTag(text)
	}
	for _, c := range [...]chroma.TokenType{t, t.SubCategory(), t.Category()} {
		if tag, ok := tagOfType[c]; ok {
			return tag
		}
	}
	return nil
}

func constantTag(word string) *tags.Tag {
	switch strings.ToLower(word) {
	case "true", "false":
		return tags.Bool
	case "nil", "null", "none", "nullptr", "undefined":
		return tags.Null
	}
	return tags.Atom
}

// punctuationTag is the tag of one punctuation character. chroma has one
// Punctuation type where lezer grammars tell brackets and separators apart,
// and its coalescing joins neighbors like "){" into one token, so every
// character is tagged on its own.
func punctuationTag(r rune) *tags.Tag {
	switch r {
	case '(', ')':
		return tags.Paren
	case '[', ']':
		return tags.SquareBracket
	case '{', '}':
		return tags.Brace
	case '<', '>':
		return tags.AngleBracket
	case ',', ';':
		return tags.Separator
	case '.':
		return tags.DerefOperator
	}
	return tags.Punctuation
}

// addToken adds a token's text to d, tagged with its one tag.
func (d *document) addToken(t chroma.Token, sets *lezer.TagSets) {
	if t.Type != chroma.Punctuation {
		d.add(t.Value, soleTag(sets, tagOf(t.Type, t.Value)))
		return
	}
	for rest := t.Value; rest != ""; {
		r, size := utf8.DecodeRuneInString(rest)
		d.add(rest[:size], soleTag(sets, punctuationTag(r)))
		rest = rest[size:]
	}
}

// soleTag is the set of tag alone, nil for no tag.
func soleTag(sets *lezer.TagSets, tag *tags.Tag) *lezer.TagSet {
	if tag == nil {
		return nil
	}
	return sets.Of(tag)
}
