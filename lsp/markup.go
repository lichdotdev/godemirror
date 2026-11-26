package lsp

import (
	"regexp"
	"strings"

	"github.com/lichdotdev/godemirror"
)

// mdLink is a markdown link, whose text is kept.
var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// hoverInfo splits m into the code it opens with, the first fenced block
// of markdown, and the rest as plain text.
func hoverInfo(m Markup) godemirror.HoverInfo {
	if m.Kind != "markdown" {
		return godemirror.HoverInfo{Text: m.Value}
	}
	var code, text []string
	fence, first := false, true
	for l := range strings.SplitSeq(m.Value, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			if fence {
				first = false
			}
			fence = !fence
			continue
		}
		if fence && first && len(text) == 0 {
			code = append(code, l)
			continue
		}
		text = append(text, l)
	}
	return godemirror.HoverInfo{Code: strings.Join(code, "\n"), Text: markdownText(strings.Join(text, "\n"))}
}

// plainText is m as plain text: markdown with its fences, links and marks
// dropped, the text they hold kept.
func plainText(m Markup) string {
	if m.Kind != "markdown" {
		return m.Value
	}
	var keep []string
	for l := range strings.SplitSeq(m.Value, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "```") {
			keep = append(keep, l)
		}
	}
	return markdownText(strings.Join(keep, "\n"))
}

// markdownText drops markdown's inline marks: links keep their text, and
// backticks, strong emphasis, heading marks and escapes go. A lone * or _
// stays, being as often code (a pointer, a name) as emphasis.
func markdownText(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = strings.NewReplacer("`", "", "**", "", "__", "").Replace(s)
	var out []string
	for l := range strings.SplitSeq(s, "\n") {
		if thematicBreak(l) {
			l = ""
		}
		l = strings.TrimLeft(l, "#")
		out = append(out, strings.TrimSpace(unescape(l)))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// thematicBreak reports whether l is a markdown rule: three or more of -,
// * or _ alone.
func thematicBreak(l string) bool {
	l = strings.ReplaceAll(strings.TrimSpace(l), " ", "")
	return len(l) >= 3 && (strings.Trim(l, "-") == "" || strings.Trim(l, "*") == "" || strings.Trim(l, "_") == "")
}

// unescape drops the backslash of a markdown escape.
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("\\`*_{}[]()#+-.!<>", s[i+1]) >= 0 {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// stripSnippet is an LSP snippet as plain text: $1 and $0 dropped,
// ${1:text} and ${1|a,b|} replaced by text and a, \$ \} \\ unescaped.
func stripSnippet(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case c == '$' && i+1 < len(s) && isDigit(s[i+1]):
			for i+1 < len(s) && isDigit(s[i+1]) {
				i++
			}
		case c == '$' && i+1 < len(s) && s[i+1] == '{':
			end := closingBrace(s, i+2)
			b.WriteString(stripSnippet(placeholder(s[i+2 : end])))
			i = end
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

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

// placeholder is the text of a tabstop's body: "1:text" is text, "1|a,b|"
// is a, and "1" or a variable's name nothing.
func placeholder(body string) string {
	i := 0
	for i < len(body) && (isDigit(body[i]) || body[i] == '_' || body[i] >= 'A' && body[i] <= 'Z' || body[i] >= 'a' && body[i] <= 'z') {
		i++
	}
	rest := body[i:]
	switch {
	case strings.HasPrefix(rest, ":"):
		return rest[1:]
	case strings.HasPrefix(rest, "|"):
		choice, _, _ := strings.Cut(strings.Trim(rest, "|"), ",")
		return choice
	}
	return ""
}
