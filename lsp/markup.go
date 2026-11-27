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
