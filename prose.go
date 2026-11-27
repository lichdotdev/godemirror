package godemirror

import (
	"image"
	"image/color"
	"regexp"
	"strings"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

// A popup's prose goes through Gio's text stack in Gio's Go font, the same
// on every machine; its code (inline, fenced, a signature) goes through
// glyphs in the editor's font, as the editor's own code does.

// proseTypeface is the face of a popup's prose.
const proseTypeface = "Go"

const (
	// popupMaxWidth and popupMaxHeight are the most a popup takes before
	// its text wraps, and before it scrolls.
	popupMaxWidth  = unit.Dp(560)
	popupMaxHeight = unit.Dp(420)
	// listIndent is how far a list nests per level.
	listIndent = unit.Dp(14)
	// maxProseSizes is how many measured runs of prose an editor keeps
	// before it measures afresh.
	maxProseSizes = 4096
)

// proseWords are a paragraph's words, each with the spaces after it.
var proseWords = regexp.MustCompile(`\S+\s*|\s+`)

// piece is a run of a popup's text placed in its content, or a rule.
type piece struct {
	at    image.Point
	w     int
	text  string
	color color.NRGBA
	bold  bool
	// italic and code say how text is drawn: code in the editor's font.
	italic, code bool
	rule         bool
}

// prose is a popup's text laid out: its pieces and its size.
type prose struct {
	pieces []piece
	size   image.Point
}

// proseKey is a measured run of prose.
type proseKey struct {
	text         string
	bold, italic bool
	px           int
}

// proseMetrics are the line's geometry, shared by prose and code: the
// baseline below a line's top, the line's height, and the prose's ascent.
type proseMetrics struct {
	baseline, lineH, proseAscent int
}

func (e *Editor) proseFont(bold, italic bool) font.Font {
	f := font.Font{Typeface: proseTypeface}
	if bold {
		f.Weight = font.Bold
	}
	if italic {
		f.Style = font.Italic
	}
	return f
}

// measure is the size of a run of prose and its baseline above its bottom.
func (e *Editor) measure(gtx layout.Context, s string, bold, italic bool) layout.Dimensions {
	k := proseKey{s, bold, italic, gtx.Sp(e.view.size)}
	if d, ok := e.proseSizes[k]; ok {
		return d
	}
	if e.shaper == nil {
		e.shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	}
	m := op.Record(gtx.Ops)
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	d := widget.Label{MaxLines: 1}.Layout(gtx, e.shaper, e.proseFont(bold, italic), e.view.size, s, op.CallOp{})
	m.Stop()
	if e.proseSizes == nil || len(e.proseSizes) > maxProseSizes {
		e.proseSizes = map[proseKey]layout.Dimensions{}
	}
	e.proseSizes[k] = d
	return d
}

func (e *Editor) metrics(gtx layout.Context) proseMetrics {
	d := e.measure(gtx, "Ag", false, false)
	ascent := d.Size.Y - d.Baseline
	codeAscent := e.view.r.Ascent()
	b := max(ascent, codeAscent)
	return proseMetrics{baseline: b, lineH: b + max(d.Baseline, e.f.ch-codeAscent), proseAscent: ascent}
}

// layoutProse lays blocks out at most maxW wide.
func (e *Editor) layoutProse(gtx layout.Context, blocks []mdBlock, maxW int) prose {
	l := proseLayout{e: e, gtx: gtx, m: e.metrics(gtx), maxW: maxW}
	for i, b := range blocks {
		if i > 0 {
			l.y += l.m.lineH / 3
		}
		switch b.kind {
		case ruleBlock:
			l.out.pieces = append(l.out.pieces, piece{at: image.Pt(0, l.y+l.m.lineH/3), rule: true})
			l.y += l.m.lineH * 2 / 3
		case codeBlock:
			l.code(b)
		default:
			l.paragraph(b)
		}
	}
	l.out.size.Y = l.y
	for i := range l.out.pieces {
		if l.out.pieces[i].rule {
			l.out.pieces[i].w = l.out.size.X
		}
	}
	return l.out
}

type proseLayout struct {
	e    *Editor
	gtx  layout.Context
	m    proseMetrics
	maxW int
	x, y int
	out  prose
}

func (l *proseLayout) paragraph(b mdBlock) {
	p := l.e.Theme.Popup
	fg := p.Foreground
	if b.muted {
		fg = p.Muted
	}
	left := b.indent * l.gtx.Dp(listIndent)
	l.x = left
	if b.bullet != "" {
		bullet := cmpColor(b.bulletColor, fg)
		l.place(piece{text: b.bullet, color: bullet}, l.e.measure(l.gtx, b.bullet, false, false).Size.X)
		left = l.x
	}
	for _, in := range b.inlines {
		if in.text == "\n" {
			l.x, l.y = left, l.y+l.m.lineH
			continue
		}
		for _, w := range proseWords.FindAllString(in.text, -1) {
			pc := piece{text: w, color: fg, bold: in.bold, italic: in.italic, code: in.code}
			width := l.width(pc)
			if l.x > left && l.x+l.width(piece{text: strings.TrimRight(w, " "), bold: in.bold, italic: in.italic, code: in.code}) > l.maxW {
				l.x, l.y = left, l.y+l.m.lineH
				if strings.TrimSpace(w) == "" {
					continue
				}
			}
			l.place(pc, width)
		}
	}
	l.y += l.m.lineH
}

func cmpColor(c, otherwise color.NRGBA) color.NRGBA {
	if c == (color.NRGBA{}) {
		return otherwise
	}
	return c
}

func (l *proseLayout) width(pc piece) int {
	if pc.code {
		return len(textCells(pc.text)) * l.e.f.cw
	}
	return l.e.measure(l.gtx, pc.text, pc.bold, pc.italic).Size.X
}

// place puts pc at the pen, joined to the piece before it when they share
// a style, and moves the pen past it.
func (l *proseLayout) place(pc piece, width int) {
	pc.at, pc.w = image.Pt(l.x, l.y), width
	out := &l.out.pieces
	if n := len(*out); n > 0 {
		last := &(*out)[n-1]
		if last.at.Y == pc.at.Y && last.at.X+last.w == pc.at.X && last.color == pc.color &&
			last.bold == pc.bold && last.italic == pc.italic && last.code == pc.code && !last.rule {
			last.text += pc.text
			last.w += width
			pc = *last
			*out = (*out)[:n-1]
		}
	}
	*out = append(*out, pc)
	l.x = pc.at.X + pc.w
	l.out.size.X = max(l.out.size.X, l.x)
}

// code lays a fenced block out a line a row, highlighted as its language
// when godemirror knows it.
func (l *proseLayout) code(b mdBlock) {
	theme := l.e.Theme
	lines, err := Highlight(codeFilename(b.lang, l.e.state.conf.Filename), b.code, nil, theme)
	if err != nil {
		lines = nil
		for line := range strings.SplitSeq(b.code, "\n") {
			lines = append(lines, []Run{{Text: line, Style: TagStyle{Color: theme.Foreground}}})
		}
	}
	for _, runs := range lines {
		l.x = 0
		for _, r := range runs {
			pc := piece{text: r.Text, color: r.Style.Color, bold: r.Style.Bold, code: true}
			l.place(pc, l.width(pc))
		}
		l.y += l.m.lineH
	}
}

// codeLanguages are the names a fence gives a language whose file
// extension is not the name itself.
var codeLanguages = map[string]string{
	"golang": "go", "javascript": "js", "typescript": "ts", "python": "py", "rust": "rs",
	"shell": "sh", "bash": "sh", "kotlin": "kt", "markdown": "md", "c++": "cpp", "yaml": "yml",
}

// codeFilename is a file name that picks lang's highlighting: the editor's
// own file for a fence with no language.
func codeFilename(lang, own string) string {
	lang = strings.ToLower(lang)
	if lang == "" {
		return own
	}
	if ext, ok := codeLanguages[lang]; ok {
		lang = ext
	}
	return "code." + lang
}

// paintProse draws p in box, scrolled down by scroll pixels, inside its
// padding.
func (e *Editor) paintProse(gtx layout.Context, p prose, box image.Rectangle, scroll int) {
	inner := box.Inset(1)
	defer clip.Rect(inner).Push(gtx.Ops).Pop()
	m := e.metrics(gtx)
	origin := box.Min.Add(image.Pt(gtx.Dp(popupPadX), gtx.Dp(popupPadY)-scroll))
	for _, pc := range p.pieces {
		at := origin.Add(pc.at)
		if at.Y > inner.Max.Y || at.Y+m.lineH < inner.Min.Y {
			continue
		}
		switch {
		case pc.rule:
			r := image.Rect(inner.Min.X, at.Y, inner.Max.X, at.Y+1)
			paint.FillShape(gtx.Ops, e.Theme.Popup.Border, clip.Rect(r).Op())
		case pc.code:
			at.Y += m.baseline - e.view.r.Ascent()
			e.view.paintTextSpans(gtx, []textSpan{{text: pc.text, color: pc.color, bold: pc.bold}}, at, inner)
		default:
			at.Y += m.baseline - m.proseAscent
			e.paintProseRun(gtx, pc, at)
		}
	}
}

func (e *Editor) paintProseRun(gtx layout.Context, pc piece, at image.Point) {
	defer op.Offset(at).Push(gtx.Ops).Pop()
	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: pc.color}.Add(gtx.Ops)
	material := m.Stop()
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	widget.Label{MaxLines: 1}.Layout(gtx, e.shaper, e.proseFont(pc.bold, pc.italic), e.view.size, pc.text, material)
}
