package lsp

import "strings"

// markdownEscapes are the characters Markdown reads as markup.
var markdownEscapes = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`, "<", `\<`, "#", `\#`,
)

// markdown is m as Markdown: plaintext with its markup characters escaped.
func markdown(m Markup) string {
	if m.Kind == "markdown" {
		return m.Value
	}
	return markdownEscapes.Replace(m.Value)
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
