package godemirror

// Run is a stretch of one line of highlighted text drawn in one style.
type Run struct {
	Text  string
	Style TagStyle
}

// Highlight is text colored as a View with theme colors it, for a caller
// that draws the text itself, a code block in Markdown, say: the language
// picked by filename as Content.Filename picks it, styles adding rules as
// Content.StyleTags do. It returns the runs of each line, a line's tabs
// expanded to the View's stops. Each Run's Style has its Tags left empty and
// a Color, theme.Foreground where no spec gives one. A filename no language
// claims gives each line one run in theme.Foreground. Highlighting runs on the
// calling goroutine.
func Highlight(filename, text string, styles []StyleTags, theme Theme) ([][]Run, error) {
	d, err := highlight(filename, text, styles)
	if err != nil {
		return nil, err
	}
	out := make([][]Run, len(d.lines))
	for i, cells := range d.lines {
		var runs []Run
		var buf []rune
		var cur TagStyle
		flush := func() {
			if len(buf) > 0 {
				runs = append(runs, Run{Text: string(buf), Style: cur})
				buf = buf[:0]
			}
		}
		for j, cell := range cells {
			if isTail(cell) {
				continue
			}
			s := theme.Highlight.style(d.tags[i][j], theme.Foreground)
			s.Tags = nil
			if len(buf) > 0 && !sameStyle(s, cur) {
				flush()
			}
			cur = s
			buf = append(buf, cell)
		}
		flush()
		out[i] = runs
	}
	return out, nil
}

func sameStyle(a, b TagStyle) bool {
	return a.Color == b.Color && a.Bold == b.Bold && a.Italic == b.Italic && a.Underline == b.Underline && a.Strikethrough == b.Strikethrough
}
