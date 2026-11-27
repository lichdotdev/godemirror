package godemirror

import (
	"image/color"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The Markdown a popup renders is the part of CommonMark language servers
// write: paragraphs, headings, lists, fenced code, rules, and inline code,
// bold, italic and links, a link shown as its text.

// inline is a run of a paragraph's text in one style. A "\n" run breaks
// the line.
type inline struct {
	text               string
	bold, italic, code bool
}

type blockKind int

const (
	paraBlock blockKind = iota
	codeBlock
	ruleBlock
)

// mdBlock is a block of a popup's text: a paragraph of inlines, after a
// list's bullet when it is an item; a fenced code block in lang; or a rule.
type mdBlock struct {
	kind    blockKind
	inlines []inline
	bullet  string
	// bulletColor, when set, is the bullet's color.
	bulletColor color.NRGBA
	indent      int
	muted       bool
	lang        string
	code        string
}

var (
	fenceLine   = regexp.MustCompile("^\\s{0,3}(```+|~~~+)\\s*([\\w+#.-]*)")
	headingLine = regexp.MustCompile(`^\s{0,3}#{1,6}\s+(.*?)\s*#*\s*$`)
	listLine    = regexp.MustCompile(`^(\s*)([-*+]|\d{1,9}[.)])\s+(.*)$`)
)

// parseMarkdown is src's blocks.
func parseMarkdown(src string) []mdBlock {
	var blocks []mdBlock
	var para []string
	var cur mdBlock
	flush := func() {
		if len(para) > 0 {
			cur.inlines = parseInlines(strings.Join(para, "\n"))
			blocks = append(blocks, cur)
		}
		para, cur = nil, mdBlock{}
	}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch m := fenceLine.FindStringSubmatch(l); {
		case m != nil:
			flush()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), m[1][:3]); i++ {
				code = append(code, lines[i])
			}
			blocks = append(blocks, mdBlock{kind: codeBlock, lang: m[2], code: strings.Join(code, "\n")})
		case strings.TrimSpace(l) == "":
			flush()
		case isRule(l):
			flush()
			blocks = append(blocks, mdBlock{kind: ruleBlock})
		case headingLine.MatchString(l):
			flush()
			blocks = append(blocks, mdBlock{inlines: bolded(parseInlines(headingLine.FindStringSubmatch(l)[1]))})
		case listLine.MatchString(l):
			flush()
			m := listLine.FindStringSubmatch(l)
			cur = mdBlock{bullet: bulletOf(m[2]), indent: len(m[1]) / 2}
			para = []string{m[3]}
		default:
			para = append(para, strings.TrimPrefix(strings.TrimLeft(l, " "), "> "))
		}
	}
	flush()
	return blocks
}

// isRule reports whether l is a thematic break: three or more of one of
// -, * and _, alone but for spaces.
func isRule(l string) bool {
	l = strings.ReplaceAll(strings.TrimSpace(l), " ", "")
	return len(l) >= 3 && (strings.Trim(l, "-") == "" || strings.Trim(l, "*") == "" || strings.Trim(l, "_") == "")
}

func bulletOf(marker string) string {
	if strings.ContainsAny(marker, ".)") {
		return marker + " "
	}
	return "• "
}

func bolded(in []inline) []inline {
	for i := range in {
		in[i].bold = true
	}
	return in
}

// parseInlines is a paragraph's text as inlines: a line ending in two
// spaces or a backslash breaks, other line ends are spaces.
func parseInlines(s string) []inline {
	p := inlineParser{src: s}
	p.parse()
	return p.out
}

type inlineParser struct {
	src          string
	out          []inline
	buf          strings.Builder
	bold, italic bool
}

func (p *inlineParser) emit(text string, code bool) {
	if text == "" {
		return
	}
	in := inline{text: text, bold: p.bold, italic: p.italic, code: code}
	if n := len(p.out); n > 0 && !code && !p.out[n-1].code && p.out[n-1].bold == in.bold && p.out[n-1].italic == in.italic && text != "\n" && p.out[n-1].text != "\n" {
		p.out[n-1].text += text
		return
	}
	p.out = append(p.out, in)
}

func (p *inlineParser) flushText() {
	p.emit(p.buf.String(), false)
	p.buf.Reset()
}

func (p *inlineParser) parse() {
	s := p.src
	for i := 0; i < len(s); {
		n := p.step(s, i)
		if n == 0 {
			p.buf.WriteByte(s[i])
			n = 1
		}
		i += n
	}
	p.flushText()
}

// step reads the markup at s[i:], and reports how many bytes it took, 0
// when there is none there.
func (p *inlineParser) step(s string, i int) int {
	switch c := s[i]; {
	case c == '\\' && i+1 < len(s) && s[i+1] == '\n':
		p.flushText()
		p.emit("\n", false)
		return 2
	case c == '\\' && i+1 < len(s) && strings.IndexByte("\\`*_{}[]()#+-.!<>|~", s[i+1]) >= 0:
		p.buf.WriteByte(s[i+1])
		return 2
	case c == '\n':
		if text := p.buf.String(); strings.HasSuffix(text, "  ") {
			p.buf.Reset()
			p.buf.WriteString(strings.TrimRight(text, " "))
			p.flushText()
			p.emit("\n", false)
		} else {
			p.buf.WriteByte(' ')
		}
		return 1
	case c == '`':
		return p.codeSpan(s, i)
	case c == '*' || c == '_':
		return p.emphasis(s, i)
	case c == '[' || c == '!' && i+1 < len(s) && s[i+1] == '[':
		return p.link(s, i)
	case c == '<':
		if end := strings.IndexByte(s[i:], '>'); end > 0 && strings.Contains(s[i:i+end], "://") {
			p.buf.WriteString(s[i+1 : i+end])
			return end + 1
		}
	}
	return 0
}

// codeSpan reads a run of backticks and the code up to the same run.
func (p *inlineParser) codeSpan(s string, i int) int {
	ticks := len(s[i:]) - len(strings.TrimLeft(s[i:], "`"))
	fence := s[i : i+ticks]
	end := strings.Index(s[i+ticks:], fence)
	if end < 0 {
		p.buf.WriteString(fence)
		return ticks
	}
	code := strings.ReplaceAll(s[i+ticks:i+ticks+end], "\n", " ")
	if len(code) > 2 && code[0] == ' ' && code[len(code)-1] == ' ' {
		code = code[1 : len(code)-1]
	}
	p.flushText()
	p.emit(code, true)
	return 2*ticks + end
}

// emphasis toggles bold (a pair of markers) or italic (one), when the
// marker can open or close: _ only at a word's edge, and an opening marker
// only when a closing one follows.
func (p *inlineParser) emphasis(s string, i int) int {
	c := s[i]
	n := 1
	if i+1 < len(s) && s[i+1] == c {
		n = 2
	}
	marker := s[i : i+n]
	on := &p.italic
	if n == 2 {
		on = &p.bold
	}
	prev, _ := utf8.DecodeLastRuneInString(s[:i])
	next, _ := utf8.DecodeRuneInString(s[i+n:])
	switch {
	case *on && !unicode.IsSpace(prev):
	case !*on && i+n < len(s) && !unicode.IsSpace(next) && strings.Contains(s[i+n:], marker):
		if c == '_' && (unicode.IsLetter(prev) || unicode.IsDigit(prev)) {
			return 0
		}
	default:
		return 0
	}
	p.flushText()
	*on = !*on
	return n
}

// link reads [text](target), or an image's ![alt](target), as its text.
func (p *inlineParser) link(s string, i int) int {
	open := i
	if s[i] == '!' {
		open++
	}
	close := strings.Index(s[open:], "](")
	if close < 0 {
		return 0
	}
	end := strings.IndexByte(s[open+close:], ')')
	if end < 0 {
		return 0
	}
	text := s[open+1 : open+close]
	p.flushText()
	for _, in := range parseInlines(text) {
		bold, italic := p.bold, p.italic
		p.bold, p.italic = in.bold || bold, in.italic || italic
		p.emit(in.text, in.code)
		p.bold, p.italic = bold, italic
	}
	return open + close + end + 1 - i
}

// plainBlocks is text as paragraphs, no Markdown read in it.
func plainBlocks(text string, muted bool) []mdBlock {
	var blocks []mdBlock
	for para := range strings.SplitSeq(strings.TrimSpace(text), "\n\n") {
		var ins []inline
		for j, l := range strings.Split(para, "\n") {
			if j > 0 {
				ins = append(ins, inline{text: "\n"})
			}
			ins = append(ins, inline{text: l})
		}
		blocks = append(blocks, mdBlock{inlines: ins, muted: muted})
	}
	return blocks
}
