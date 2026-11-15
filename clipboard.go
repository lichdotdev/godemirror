package godemirror

import (
	"strings"
	"sync/atomic"
)

// copiedRange and doPaste port @codemirror/view 6.43.15's functions of the
// same names in input.ts, without its clipboard filters.

// lastLinewiseCopy is the text of the last copy or cut of whole lines, made
// with nothing selected, in any editor of the process, as CodeMirror keeps
// it for every editor of a page: pasting exactly that text back pastes it as
// lines.
var lastLinewiseCopy atomic.Pointer[string]

// copied is what a copy or a cut takes: the text, joined by the line
// separator, and the ranges a cut deletes.
type copied struct {
	text     string
	ranges   []ChangeSpec
	linewise bool
}

// copiedRange is the selected text, or, with only cursors, their whole
// lines and the line breaks after them.
func (s EditorState) copiedRange() copied {
	var c copied
	var content []string
	for _, r := range s.sel.ranges {
		if !r.Empty() {
			content = append(content, s.SliceDoc(r.from, r.to))
			c.ranges = append(c.ranges, ChangeSpec{From: r.from, To: r.to})
		}
	}
	if len(content) == 0 {
		upto := -1
		for _, r := range s.sel.ranges {
			line := s.LineAt(r.from)
			if line.Number > upto {
				content = append(content, line.Text)
				c.ranges = append(c.ranges, ChangeSpec{From: line.From, To: min(len(s.doc), line.To+1)})
			}
			upto = line.Number
		}
		c.linewise = true
	}
	c.text = strings.Join(content, s.lineBreak())
	return c
}

// lineBreak is what SliceDoc joins lines with.
func (s EditorState) lineBreak() string {
	if s.conf.LineSeparator != "" {
		return s.conf.LineSeparator
	}
	return "\n"
}

// doPaste inserts input at every range: one line of it at each when it has
// as many lines as there are ranges, and above each cursor's line when it is
// what the last copy of whole lines took.
func (s EditorState) doPaste(input string, linewiseCopy *string) TransactionSpec {
	text := normalizeBreaks(input, s.conf.LineSeparator)
	lines := strings.Split(text, "\n")
	byLine := len(lines) == len(s.sel.ranges)
	linewise := linewiseCopy != nil && *linewiseCopy == text
	for _, r := range s.sel.ranges {
		linewise = linewise && r.Empty()
	}
	var spec TransactionSpec
	i := 0
	switch {
	case linewise:
		lastLine := -1
		spec = s.ChangeByRange(func(r SelectionRange) RangeChange {
			line := s.LineAt(r.from)
			if line.From == lastLine {
				return RangeChange{Range: r}
			}
			lastLine = line.From
			insert := input
			if byLine {
				insert = lines[i]
				i++
			}
			insert = normalizeBreaks(insert+s.lineBreak(), s.conf.LineSeparator)
			return RangeChange{
				Changes: []ChangeSpec{{From: line.From, To: line.From, Insert: insert}},
				Range:   cursorAssoc(r.from+len(insert), -1),
			}
		})
	case byLine:
		spec = s.ChangeByRange(func(r SelectionRange) RangeChange {
			line := lines[i]
			i++
			return RangeChange{
				Changes: []ChangeSpec{{From: r.from, To: r.to, Insert: line}},
				Range:   cursorAssoc(r.from+len(line), -1),
			}
		})
	default:
		spec = s.ReplaceSelection(text)
	}
	spec.UserEvent, spec.ScrollIntoView = "input.paste", true
	return spec
}
