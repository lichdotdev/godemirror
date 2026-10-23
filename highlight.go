package godemirror

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/lichdotdev/godemirror/internal/glyphs"
	"github.com/lichdotdev/godemirror/internal/lezer"
	"github.com/lichdotdev/godemirror/internal/lezer/css"
	"github.com/lichdotdev/godemirror/internal/lezer/golang"
	"github.com/lichdotdev/godemirror/internal/lezer/html"
	"github.com/lichdotdev/godemirror/internal/lezer/java"
	"github.com/lichdotdev/godemirror/internal/lezer/javascript"
	"github.com/lichdotdev/godemirror/internal/lezer/python"
	"github.com/lichdotdev/godemirror/internal/lezer/rust"
	"github.com/lichdotdev/godemirror/internal/stream"
	"github.com/lichdotdev/godemirror/internal/stream/kotlin"
	"github.com/lichdotdev/godemirror/internal/stream/shell"
)

// document is text cut into lines of cells, tabs expanded to tabSize stops
// and wide characters to two cells, with the highlighting tags of every cell. It keeps
// each line's own runes too, and the cell each of them starts at, so what is
// copied out is the text as it was.
type document struct {
	lines  [][]rune
	tags   [][]*lezer.TagSet
	source [][]rune
	cellOf [][]int
}

func (d *document) add(text string, tags *lezer.TagSet) {
	for _, r := range text {
		last := len(d.lines) - 1
		if r == '\n' {
			d.lines = append(d.lines, nil)
			d.tags = append(d.tags, nil)
			d.source = append(d.source, nil)
			d.cellOf = append(d.cellOf, nil)
			continue
		}
		d.source[last] = append(d.source[last], r)
		d.cellOf[last] = append(d.cellOf[last], len(d.lines[last]))
		start := len(d.lines[last])
		if r == '\t' {
			for range tabSize - start%tabSize {
				d.lines[last] = append(d.lines[last], ' ')
			}
		} else {
			d.lines[last] = appendCells(d.lines[last], r)
		}
		for range len(d.lines[last]) - start {
			d.tags[last] = append(d.tags[last], tags)
		}
	}
}

// appendCells appends the cells r takes: r itself, and for a wide character
// a second cell holding -r, so a cell always says which character it is part
// of and whether it is that character's first.
func appendCells(cells []rune, r rune) []rune {
	cells = append(cells, r)
	if glyphs.Width(r) == 2 {
		cells = append(cells, -r)
	}
	return cells
}

// textCells is s laid out in cells, as a line is with no tabs.
func textCells(s string) []rune {
	var cells []rune
	for _, r := range s {
		cells = appendCells(cells, r)
	}
	return cells
}

// isTail reports whether a cell is the second half of a wide character.
func isTail(cell rune) bool { return cell < 0 }

// charAt is the character cell is part of.
func charAt(cell rune) rune {
	if isTail(cell) {
		return -cell
	}
	return cell
}

// sourceText is the original text of cells [from, to) of line: every rune that
// starts inside the span, a tab whole however many of its cells are in it.
func (d *document) sourceText(line, from, to int) string {
	cells := d.cellOf[line]
	first, _ := slices.BinarySearch(cells, from)
	last, _ := slices.BinarySearch(cells, to)
	return string(d.source[line][first:last])
}

// normalize is the text a document is built from: LF line ends, and no empty
// last line for a trailing newline.
func normalize(text string) string {
	return strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

func newDocument() *document {
	return &document{lines: [][]rune{nil}, tags: [][]*lezer.TagSet{nil}, source: [][]rune{nil}, cellOf: [][]int{nil}}
}

// plain builds the document of text with every cell untagged. Its cells are
// exactly highlight's, so one can stand in for the other.
func plain(text string) *document {
	d := newDocument()
	d.add(normalize(text), nil)
	return d
}

// lezerLanguage is a language highlighted by a port of its Lezer grammar:
// the name of its @lezer package, which StyleTags names it by, and its
// parser.
type lezerLanguage struct {
	name   string
	parser func() *lezer.Parser
}

// lezerLanguages are the Lezer languages by file extension, as
// @codemirror/language-data matches them.
var lezerLanguages = map[string]lezerLanguage{
	".css":        {"css", css.Parser},
	".go":         {"go", golang.Parser},
	".html":       {"html", html.Parser},
	".htm":        {"html", html.Parser},
	".handlebars": {"html", html.Parser},
	".hbs":        {"html", html.Parser},
	".java":       {"java", java.Parser},
	".js":         {"javascript", javascript.Parser},
	".mjs":        {"javascript", javascript.Parser},
	".cjs":        {"javascript", javascript.Parser},
	".jsx":        {"javascript", javascript.JSX},
	".ts":         {"javascript", javascript.TypeScript},
	".mts":        {"javascript", javascript.TypeScript},
	".cts":        {"javascript", javascript.TypeScript},
	".tsx":        {"javascript", javascript.TSX},
	".py":         {"python", python.Parser},
	".pyw":        {"python", python.Parser},
	".bzl":        {"python", python.Parser},
	".build":      {"python", python.Parser},
	".rs":         {"rust", rust.Parser},
}

// streamModes are the languages highlighted by a port of their
// @codemirror/legacy-modes mode, by file extension, as
// @codemirror/language-data matches them.
var streamModes = map[string]func() stream.Mode{
	".kt":   kotlin.New,
	".kts":  kotlin.New,
	".sh":   shell.New,
	".ksh":  shell.New,
	".bash": shell.New,
}

// streamModeFor is the mode for filename: by its name for the one name
// @codemirror/language-data matches whole, by its extension otherwise.
func streamModeFor(filename string) (func() stream.Mode, bool) {
	if filepath.Base(filename) == "PKGBUILD" {
		return shell.New, true
	}
	mode, ok := streamModes[strings.ToLower(filepath.Ext(filename))]
	return mode, ok
}

// highlight builds the document for a file, colored by the Lezer grammar
// or the stream mode it matches, or else by the chroma lexer its name
// matches. A file none claims is shown plain, as CodeMirror shows a file
// with no language.
func highlight(filename, text string, styles []StyleTags) (*document, error) {
	d, _, err := highlightAfter(filename, text, styles, nil)
	return d, err
}

// highlightAfter is highlight for a document that follows prev, the parse
// of the document before it (nil for none): a file Lezer parses with the
// same grammar is reparsed incrementally, as CodeMirror reparses after an
// edit. It also returns the parse the next document can follow, nil for a
// file Lezer does not parse. prev's tree shares nodes with the new one and
// must not be used again. styles add rules to the Lezer grammars they name.
func highlightAfter(filename, text string, styles []StyleTags, prev *lezerParse) (*document, *lezerParse, error) {
	parser, err := lezerParser(filename, styles)
	if err != nil {
		return nil, nil, fmt.Errorf("highlight %s: %w", filename, err)
	}
	if parser != nil {
		d, parsed := highlightLezer(parser, normalize(text), prev)
		return d, parsed, nil
	}
	if mode, ok := streamModeFor(filename); ok {
		return highlightStream(mode(), normalize(text)), nil, nil
	}
	lexer := lexers.Match(filename)
	if lexer == nil {
		return plain(text), nil, nil
	}
	d, err := highlightChroma(lexer, normalize(text))
	if err != nil {
		return nil, nil, fmt.Errorf("highlight %s: %w", filename, err)
	}
	return d, nil, nil
}

// highlightApart is highlight for text whose lines from each of breaks on
// are highlighted on their own, so that what the lines before a break leave
// open (a block comment, a string) ends there. It parses from scratch.
func highlightApart(filename, text string, breaks []int, styles []StyleTags) (*document, error) {
	lines := strings.Split(normalize(text), "\n")
	d := &document{}
	from := 0
	for _, to := range append(breaks, len(lines)) {
		if to <= from {
			continue
		}
		// The line end keeps an empty last line, which normalize drops.
		part, err := highlight(filename, strings.Join(lines[from:to], "\n")+"\n", styles)
		if err != nil {
			return nil, err
		}
		d.lines, d.tags = append(d.lines, part.lines...), append(d.tags, part.tags...)
		d.source, d.cellOf = append(d.source, part.source...), append(d.cellOf, part.cellOf...)
		from = to
	}
	return d, nil
}

// highlightChroma builds the document of text with the tags tagmap.go gives
// lexer's tokens.
func highlightChroma(lexer chroma.Lexer, text string) (*document, error) {
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return nil, err
	}
	d, sets := newDocument(), lezer.NewTagSets()
	for t := it(); t != chroma.EOF; t = it() {
		d.addToken(t, sets)
	}
	// Lexers that ensure a final newline hand one back the text never had.
	lines := strings.Count(text, "\n") + 1
	d.lines, d.tags = d.lines[:lines], d.tags[:lines]
	d.source, d.cellOf = d.source[:lines], d.cellOf[:lines]
	return d, nil
}

// highlightLezer builds the document of text with the tags CodeMirror gives
// it through parser's grammar, reparsing incrementally after prev.
func highlightLezer(parser *lezer.Parser, text string, prev *lezerParse) (*document, *lezerParse) {
	units := utf16.Encode([]rune(text))
	parsed := &lezerParse{parser: parser, units: units, tree: reparse(parser, units, prev)}
	w := &unitWriter{d: newDocument(), text: text}
	lezer.Highlight(parsed.tree, units, lezer.NewTagSets(), func(from, to int, tags *lezer.TagSet) {
		w.writeTo(from, nil)
		w.writeTo(to, tags)
	})
	w.writeTo(len(units), nil)
	return w.d, parsed
}

// lezerParse is a Lezer parse of a document's text, in UTF-16 code units.
type lezerParse struct {
	parser *lezer.Parser
	units  []uint16
	tree   *lezer.Tree
}

// reparse parses units with parser, reusing what it can of prev's tree when
// prev was parsed by the same grammar. The change between the two texts is
// taken to be the one stretch between their common prefix and suffix.
func reparse(parser *lezer.Parser, units []uint16, prev *lezerParse) *lezer.Tree {
	if prev == nil || prev.parser != parser {
		return parser.ParseUnits(units, nil, nil)
	}
	if slices.Equal(prev.units, units) {
		return prev.tree
	}
	fragments := lezer.ApplyChanges(lezer.AddTree(prev.tree, nil, false), []lezer.ChangedRange{changedRange(prev.units, units)})
	return parser.ParseUnits(units, nil, fragments)
}

// changedRange is the stretch of old that differs from new, between their
// longest common prefix and the longest common suffix after it.
func changedRange(old, new []uint16) lezer.ChangedRange {
	prefix := 0
	for prefix < min(len(old), len(new)) && old[prefix] == new[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < min(len(old), len(new))-prefix && old[len(old)-1-suffix] == new[len(new)-1-suffix] {
		suffix++
	}
	return lezer.ChangedRange{FromA: prefix, ToA: len(old) - suffix, FromB: prefix, ToB: len(new) - suffix}
}

// highlightStream builds the document of text with the tags CodeMirror
// gives it through mode.
func highlightStream(mode stream.Mode, text string) *document {
	units := utf16.Encode([]rune(text))
	w := &unitWriter{d: newDocument(), text: text}
	stream.Highlight(units, mode, lezer.NewTagSets(), func(from, to int, tags *lezer.TagSet) {
		w.writeTo(from, nil)
		w.writeTo(to, tags)
	})
	w.writeTo(len(units), nil)
	return w.d
}

// unitWriter adds text to a document by UTF-16 offsets, the positions a
// Lezer tree and a stream mode count in.
type unitWriter struct {
	d            *document
	text         string
	byteAt, unit int
}

// writeTo adds the text from where the last call stopped up to the given
// UTF-16 offset, tagged tags. A character whose first unit is before the
// offset goes whole.
func (w *unitWriter) writeTo(unit int, tags *lezer.TagSet) {
	start := w.byteAt
	for w.unit < unit && w.byteAt < len(w.text) {
		r, size := utf8.DecodeRuneInString(w.text[w.byteAt:])
		w.byteAt += size
		w.unit += utf16.RuneLen(r)
	}
	w.d.add(w.text[start:w.byteAt], tags)
}
