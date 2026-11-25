package godemirror

import (
	"image"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
)

// Content is everything a View shows of one document. Labels and
// Decorations are indexed by document line (0-based); a line past the end of
// either has no label or decoration.
type Content struct {
	// Filename picks the language Text is highlighted as, by its name or
	// extension; a name no lexer claims is shown plain.
	Filename string
	// Text is the document. Lines end in LF or CRLF, and a final line end
	// does not start an empty line.
	Text string
	// Labels replace the gutter's line numbers; nil numbers the lines from 1.
	Labels      []string
	Decorations []LineDecoration
	// Widgets are drawn on lines, and Blocks between them, in slice order.
	Widgets []LineWidget
	Blocks  []BlockWidget
	// StyleTags add rules to the Lezer grammars they name, for finer tags
	// than CodeMirror's own. A StyleTags naming no Lezer grammar, or a
	// rule whose path does not parse, makes Layout return an error.
	StyleTags []StyleTags

	// breaks are the lines that do not follow the line above them, a diff's
	// rows either side of a gap: highlighting starts over at each.
	breaks []int
}

// pending is highlighting being worked out off the UI goroutine: for what is
// on screen, or, when content is set, for the content that replaces it.
type pending struct {
	done    chan highlighted
	content *Content
}

type highlighted struct {
	doc *document
	err error
}

// SetContent shows c from its top, with nothing selected. Its text shows at
// once; its highlighting is worked out off the UI goroutine and swapped in by
// a later Layout.
func (v *View) SetContent(c Content) {
	v.showContent(c, plain(c.Text))
	v.scroll = image.Point{}
	v.pending = v.highlight(c, nil)
}

// UpdateContent replaces what is shown with a new version of it (a diff with
// lines pulled in, say), keeping the scroll position. The current content stays
// on screen until c's highlighting is ready, and then c takes its place
// whole, so text, labels and decorations never show out of step. Until then,
// whatever reads or addresses the view by line (SelectedLines, Selection,
// Select, SelectLines, ScrollToLine, LineCount, LineText) works on the old
// content. The selection is dropped, since its lines may no longer be where
// they were.
func (v *View) UpdateContent(c Content) {
	v.pending = v.highlight(c, &c)
}

// LineCount is the number of lines shown. A final newline does not start a
// line of its own. Between UpdateContent and the swap it describes (see
// UpdateContent), it is the old content's.
func (v *View) LineCount() int { return len(v.doc.lines) }

// LineText is line i (0-based) as written, tabs and all, without its line
// end. i must be in [0, LineCount()). Like LineCount, it reads the content
// on screen, which UpdateContent replaces only once highlighted.
func (v *View) LineText(i int) string { return string(v.doc.source[i]) }

// highlight works out c's highlighting off the UI goroutine. Each run waits
// for the run before it and reparses after its parse, so runs go one at a
// time, in the order they were asked for, and a parse's tree, which the
// next parse shares nodes with and may change, is only ever used by one. A
// run asked for again before it starts hands the parse before it on
// untouched, since nothing will show its result.
func (v *View) highlight(c Content, replacement *Content) *pending {
	p := &pending{done: make(chan highlighted, 1), content: replacement}
	prev, next := v.lastParse, make(chan *lezerParse, 1)
	v.lastParse = next
	run := v.highlightRuns.Add(1)
	go func() {
		var after *lezerParse
		if prev != nil {
			after = <-prev
		}
		if v.highlightRuns.Load() != run {
			next <- after
			return
		}
		if len(c.breaks) > 0 {
			next <- after
			doc, err := highlightApart(c.Filename, c.Text, c.breaks, c.StyleTags)
			p.done <- highlighted{doc, err}
			return
		}
		doc, parsed, err := highlightAfter(c.Filename, c.Text, c.StyleTags, after)
		next <- parsed
		p.done <- highlighted{doc, err}
	}()
	return p
}

// showEdit shows c, an edit of what is on screen that left its first keepTop
// and last keepBottom lines alone and made middle of the lines between,
// keeping the scroll and the selection. Only middle's lines are built
// again: the others keep their cells, colors and wrapping, and their row
// images, renumbered, until c's own highlighting is ready.
func (v *View) showEdit(c Content, keepTop, keepBottom int, middle string) {
	old, mid := v.doc, newDocument()
	mid.add(middle, nil)
	tail := len(old.lines) - keepBottom
	v.doc = &document{
		lines:  splice(old.lines, keepTop, tail, mid.lines),
		tags:   splice(old.tags, keepTop, tail, mid.tags),
		source: splice(old.source, keepTop, tail, mid.source),
		cellOf: splice(old.cellOf, keepTop, tail, mid.cellOf),
	}
	if v.wrapCols >= 0 {
		wraps := make([][]int, len(mid.lines))
		for i, l := range mid.lines {
			wraps[i] = wrapLine(l, v.wrapCols)
		}
		v.wraps = splice(v.wraps, keepTop, tail, wraps)
	}
	v.renumberRows(keepTop, tail, len(mid.lines)-(tail-keepTop))
	v.content = c
	v.longest = 0
	for _, l := range v.doc.lines {
		v.longest = max(v.longest, len(l))
	}
	v.measureLabels()
	v.findMatches()
	v.pending = v.highlight(c, nil)
}

// splice is s with s[from:to] replaced by mid, in a new slice.
func splice[T any](s []T, from, to int, mid []T) []T {
	out := make([]T, 0, len(s)-(to-from)+len(mid))
	out = append(out, s[:from]...)
	out = append(out, mid...)
	return append(out, s[to:]...)
}

// renumberRows keeps the row images of the lines before from, those from
// to on renumbered by delta, and drops the others and the gutter's, as an
// edit of the lines [from, to) leaves them.
func (v *View) renumberRows(from, to, delta int) {
	rows := make(map[rowKey]*rowImage, len(v.rows))
	for k, img := range v.rows {
		switch {
		case k.line < from:
			rows[k] = img
		case k.line >= to:
			k.line += delta
			rows[k] = img
		}
	}
	v.rows = rows
	v.gutter = map[gutterKey]*rowImage{}
}

func (v *View) showContent(c Content, doc *document) {
	v.content = c
	v.sel = selection{}
	v.wrapCols = -1
	v.setDocument(doc)
	v.findMatches()
}

func (v *View) setDocument(doc *document) {
	v.doc = doc
	v.longest = 0
	for _, l := range doc.lines {
		v.longest = max(v.longest, len(l))
	}
	v.measureLabels()
	v.dropRows()
}

// takeHighlight swaps in finished highlighting, or, while it is still being
// worked out, asks for another frame to look again.
func (v *View) takeHighlight(gtx layout.Context) error {
	if v.pending == nil {
		return nil
	}
	select {
	case h := <-v.pending.done:
		p := v.pending
		v.pending = nil
		if h.err != nil {
			return h.err
		}
		if p.content != nil {
			v.showContent(*p.content, h.doc)
			return nil
		}
		v.setDocument(h.doc)
	default:
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(highlightPoll)})
	}
	return nil
}

func (v *View) label(line int) string {
	switch {
	case v.content.Labels == nil && v.formatNumber != nil:
		return v.formatNumber(line + 1)
	case v.content.Labels == nil:
		return strconv.Itoa(line + 1)
	case line < len(v.content.Labels):
		return v.content.Labels[line]
	}
	return ""
}

func (v *View) measureLabels() {
	if v.content.Labels == nil {
		v.labelWidth = len(strconv.Itoa(len(v.doc.lines)))
		return
	}
	v.labelWidth = widestLabel(v.content.Labels)
}

// widestLabel is the width in cells of the widest of labels, at least 1.
func widestLabel(labels []string) int {
	width := 1
	for _, l := range labels {
		width = max(width, len(textCells(l)))
	}
	return width
}
