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

// lezerParsers are the languages highlighted by a port of their Lezer
// grammar, by file extension, as @codemirror/language-data matches them.
var lezerParsers = map[string]func() *lezer.Parser{
	".css":        css.Parser,
	".go":         golang.Parser,
	".html":       html.Parser,
	".htm":        html.Parser,
	".handlebars": html.Parser,
	".hbs":        html.Parser,
	".java":       java.Parser,
	".js":         javascript.Parser,
	".mjs":        javascript.Parser,
	".cjs":        javascript.Parser,
	".jsx":        javascript.JSX,
	".ts":         javascript.TypeScript,
	".mts":        javascript.TypeScript,
	".cts":        javascript.TypeScript,
	".tsx":        javascript.TSX,
	".py":         python.Parser,
	".pyw":        python.Parser,
	".bzl":        python.Parser,
	".build":      python.Parser,
	".rs":         rust.Parser,
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
func highlight(filename, text string) (*document, error) {
	if parser, ok := lezerParsers[strings.ToLower(filepath.Ext(filename))]; ok {
		return highlightLezer(parser(), normalize(text)), nil
	}
	if mode, ok := streamModeFor(filename); ok {
		return highlightStream(mode(), normalize(text)), nil
	}
	lexer := lexers.Match(filename)
	if lexer == nil {
		return plain(text), nil
	}
	d, err := highlightChroma(lexer, normalize(text))
	if err != nil {
		return nil, fmt.Errorf("highlight %s: %w", filename, err)
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
// it through parser's grammar.
func highlightLezer(parser *lezer.Parser, text string) *document {
	units := utf16.Encode([]rune(text))
	w := &unitWriter{d: newDocument(), text: text}
	lezer.Highlight(parser.ParseUnits(units, nil), lezer.NewTagSets(), func(from, to int, tags *lezer.TagSet) {
		w.writeTo(from, nil)
		w.writeTo(to, tags)
	})
	w.writeTo(len(units), nil)
	return w.d
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
