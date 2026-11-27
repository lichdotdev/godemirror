package godemirror

import (
	"cmp"
	"slices"
	"strings"

	"gioui.org/io/key"
)

// Snippets are the syntax LSP and VS Code share, which CodeMirror's
// snippet() reads too: $1 and ${1:text} are fields, visited in order of
// their number with Tab, $0 the last; ${1|a,b|} is a field holding a;
// ${NAME} and ${NAME:text} are variables, inserted as nothing or as text;
// \$, \} and \\ are the characters themselves.

// snippetField is a field of a snippet: its number and its range.
type snippetField struct {
	index    int
	from, to int
}

// snippetSession is the snippet whose fields Tab visits: its fields, the
// number of the one visited, and the range the whole snippet covers.
type snippetSession struct {
	fields     []snippetField
	at         int
	start, end int
}

// InsertSnippet replaces from, to with snippet, positions in the document
// and the snippet in the syntax above, each of its lines after the first
// indented as the line from is on; it selects the first field, and Tab and
// Shift+Tab then go from field to field until the last, $0 or the
// snippet's end. Escape, or a selection out of the snippet, ends it. A
// snippet with no fields is text, and leaves a snippet it goes in on. It
// fails as Dispatch fails.
func (e *Editor) InsertSnippet(from, to int, snippet string) error {
	l := e.state.LineAt(from)
	indent := l.Text[:len(l.Text)-len(strings.TrimLeft(l.Text, " \t"))]
	text, fields := parseSnippet(snippet, indent)
	if err := e.Dispatch(TransactionSpec{Changes: []ChangeSpec{{From: from, To: to, Insert: text}},
		ScrollIntoView: true, UserEvent: "input.complete"}); err != nil {
		return err
	}
	if len(fields) == 0 {
		// Text alone: a snippet it was typed in goes on.
		end := from + len(text)
		e.mustDispatch(TransactionSpec{Selection: SingleSelection(end, end)})
		return nil
	}
	for i := range fields {
		fields[i].from += from
		fields[i].to += from
	}
	e.snippet = &snippetSession{fields: fields, at: -1, start: from, end: from + len(text)}
	e.nextField(1)
	return nil
}

// order is the numbers of the session's fields as Tab visits them, $0
// last.
func (s *snippetSession) order() []int {
	var out []int
	for _, f := range s.fields {
		if !slices.Contains(out, f.index) {
			out = append(out, f.index)
		}
	}
	slices.SortFunc(out, func(a, b int) int {
		if (a == 0) != (b == 0) {
			return cmp.Compare(b, a)
		}
		return cmp.Compare(a, b)
	})
	return out
}

// nextField selects the field dir steps from the one visited: reaching $0,
// or going past the last field, ends the snippet there; going back from the
// first does nothing.
func (e *Editor) nextField(dir int) {
	s := e.snippet
	order := s.order()
	i := slices.Index(order, s.at) + dir
	switch {
	case i < 0:
		return
	case i >= len(order):
		e.snippet = nil
		e.mustDispatch(TransactionSpec{Selection: SingleSelection(s.end, s.end), UserEvent: "select"})
		return
	}
	s.at = order[i]
	var ranges []SelectionRange
	for _, f := range s.fields {
		if f.index == s.at {
			ranges = append(ranges, Range(f.from, f.to))
		}
	}
	if !e.state.conf.AllowMultipleSelections {
		ranges = ranges[:1]
	}
	e.mustDispatch(TransactionSpec{Selection: NewSelection(ranges, 0), ScrollIntoView: true, UserEvent: "select"})
	if s.at == 0 {
		e.snippet = nil
	}
}

// snippetKey runs Tab, Shift+Tab or Escape on the snippet, and reports
// whether it took the key.
func (e *Editor) snippetKey(ev key.Event) bool {
	if e.snippet == nil {
		return false
	}
	switch {
	case ev.Name == key.NameTab && ev.Modifiers == 0:
		e.nextField(1)
	case ev.Name == key.NameTab && ev.Modifiers == key.ModShift:
		e.nextField(-1)
	case ev.Name == key.NameEscape && ev.Modifiers == 0:
		e.snippet = nil
	default:
		return false
	}
	return true
}

// mapSnippet moves the snippet's fields through c, a field growing with
// what is typed at its ends, and ends the snippet when the selection
// leaves it.
func (e *Editor) mapSnippet(t Transaction) {
	s := e.snippet
	if s == nil {
		return
	}
	for i, f := range s.fields {
		s.fields[i].from, s.fields[i].to = t.Changes.MapPos(f.from, -1), t.Changes.MapPos(f.to, 1)
	}
	s.start, s.end = t.Changes.MapPos(s.start, -1), t.Changes.MapPos(s.end, 1)
	for _, r := range t.State.sel.ranges {
		if r.from < s.start || r.to > s.end {
			e.snippet = nil
			return
		}
	}
}

// parseSnippet is snippet's text, each line after the first indented by
// indent, and its fields, in bytes of that text.
func parseSnippet(snippet, indent string) (string, []snippetField) {
	p := snippetParser{indent: indent}
	p.parse(snippet)
	return p.out.String(), p.fields
}

type snippetParser struct {
	indent string
	out    strings.Builder
	fields []snippetField
}

func (p *snippetParser) parse(s string) {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && i+1 < len(s) && strings.IndexByte(`$}\`, s[i+1]) >= 0:
			i++
			p.out.WriteByte(s[i])
		case c == '\n':
			p.out.WriteString("\n" + p.indent)
		case c == '$' && i+1 < len(s) && isDigit(s[i+1]):
			j := i + 1
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			p.field(atoi(s[i+1:j]), "")
			i = j - 1
		case c == '$' && i+1 < len(s) && s[i+1] == '{':
			end := closingBrace(s, i+2)
			p.braced(s[i+2 : end])
			i = end
		default:
			p.out.WriteByte(c)
		}
	}
}

// braced reads the body of ${...}: a field with its placeholder or
// choices, or a variable with its default.
func (p *snippetParser) braced(body string) {
	n := 0
	for n < len(body) && isDigit(body[n]) {
		n++
	}
	if n == 0 {
		_, def, _ := strings.Cut(body, ":")
		p.parse(def)
		return
	}
	rest := body[n:]
	switch {
	case strings.HasPrefix(rest, ":"):
		p.field(atoi(body[:n]), rest[1:])
	case strings.HasPrefix(rest, "|"):
		choice, _, _ := strings.Cut(strings.Trim(rest, "|"), ",")
		p.field(atoi(body[:n]), choice)
	default:
		p.field(atoi(body[:n]), "")
	}
}

// field writes a field of index holding placeholder, itself a snippet.
func (p *snippetParser) field(index int, placeholder string) {
	from := p.out.Len()
	p.parse(placeholder)
	p.fields = append(p.fields, snippetField{index: index, from: from, to: p.out.Len()})
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func atoi(s string) int {
	n := 0
	for _, c := range []byte(s) {
		n = n*10 + int(c-'0')
	}
	return n
}

// closingBrace is the index of the brace closing the one before from,
// nested ones and escapes skipped; len(s) when there is none.
func closingBrace(s string, from int) int {
	depth := 1
	for i := from; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return len(s)
}
