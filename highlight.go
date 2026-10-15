package godemirror

import (
	"fmt"
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/lichdotdev/godemirror/internal/glyphs"
	"github.com/lichdotdev/godemirror/tags"
)

// document is text cut into lines of cells, tabs expanded to tabSize stops
// and wide characters to two cells, with the highlighting tag of every cell. It keeps
// each line's own runes too, and the cell each of them starts at, so what is
// copied out is the text as it was.
type document struct {
	lines  [][]rune
	tags   [][]*tags.Tag
	source [][]rune
	cellOf [][]int
}

func (d *document) add(text string, tag *tags.Tag) {
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
			d.tags[last] = append(d.tags[last], tag)
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
	return &document{lines: [][]rune{nil}, tags: [][]*tags.Tag{nil}, source: [][]rune{nil}, cellOf: [][]int{nil}}
}

// plain builds the document of text with every cell untagged. Its cells are
// exactly highlight's, so one can stand in for the other.
func plain(text string) *document {
	d := newDocument()
	d.add(normalize(text), nil)
	return d
}

// highlight builds the document for a file, colored by the lexer its name
// matches. A file no lexer claims is shown plain, as CodeMirror shows a file
// with no language.
func highlight(filename, text string) (*document, error) {
	lexer := lexers.Match(filename)
	if lexer == nil {
		return plain(text), nil
	}
	text = normalize(text)
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return nil, fmt.Errorf("highlight %s: %w", filename, err)
	}
	d := newDocument()
	for t := it(); t != chroma.EOF; t = it() {
		d.addToken(t)
	}
	// Lexers that ensure a final newline hand one back the text never had.
	lines := strings.Count(text, "\n") + 1
	d.lines, d.tags = d.lines[:lines], d.tags[:lines]
	d.source, d.cellOf = d.source[:lines], d.cellOf[:lines]
	return d, nil
}
