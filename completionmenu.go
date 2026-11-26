package godemirror

import (
	"image"
	"image/color"
	"strings"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/tags"
)

const (
	// completionRows is how many completions the menu shows at once.
	completionRows = 9
	// completionRowPad is the room above and below a row's text,
	// completionBadgeGap the room after the badge and completionDetailGap
	// the least between a label and its detail.
	completionRowPad    = unit.Dp(2)
	completionBadgeGap  = unit.Dp(8)
	completionDetailGap = unit.Dp(24)
	// completionMaxDetail is how many cells of an item's detail the menu
	// shows, and completionDocCols how wide the documentation beside it
	// runs.
	completionMaxDetail = 40
	completionDocCols   = 48
)

// kindTags are the highlighting tags whose color a kind's badge takes.
var kindTags = map[CompletionKind]*tags.Tag{
	CompletionClass:     tags.ClassName,
	CompletionConstant:  tags.Constant(tags.Name),
	CompletionEnum:      tags.TypeName,
	CompletionFunction:  tags.Function(tags.VariableName),
	CompletionInterface: tags.TypeName,
	CompletionKeyword:   tags.Keyword,
	CompletionMethod:    tags.Function(tags.PropertyName),
	CompletionNamespace: tags.Namespace,
	CompletionProperty:  tags.PropertyName,
	CompletionType:      tags.TypeName,
	CompletionVariable:  tags.VariableName,
}

// badgeColor is the color of k's badge: its tag's in the theme, the muted
// popup text's for a kind with none.
func (e *Editor) badgeColor(k CompletionKind) color.NRGBA {
	if t, ok := kindTags[k]; ok {
		return e.Theme.tagColor(t)
	}
	return e.Theme.Popup.Muted
}

// menuLayout is where the menu's parts go, in pixels.
type menuLayout struct {
	box                       image.Rectangle
	rowH, padX, badge, labelX int
}

// paintCompletion draws the menu of completions under the cursor, and the
// chosen one's documentation beside it.
func (e *Editor) paintCompletion(gtx layout.Context) {
	c := &e.completion
	if !c.open || len(c.shown) == 0 {
		return
	}
	m := e.menuLayout(gtx)
	e.paintPopupBox(gtx, m.box)
	typed, _ := e.typedSince()
	for row := range min(completionRows, len(c.shown)-c.top) {
		i := c.top + row
		e.paintCompletionRow(gtx, m, row, c.items[c.shown[i]], typed, i == c.sel)
	}
	e.paintCompletionDoc(gtx, m.box)
}

// menuLayout places the menu under the cursor's row, its labels lined up
// with the word being completed, or above the row when there is no room
// under it.
func (e *Editor) menuLayout(gtx layout.Context) menuLayout {
	c, f := &e.completion, e.f
	m := menuLayout{rowH: f.ch + 2*gtx.Dp(completionRowPad), padX: gtx.Dp(popupPadX) / 2}
	m.badge = f.ch - 2*gtx.Dp(completionRowPad)
	m.labelX = m.padX + m.badge + gtx.Dp(completionBadgeGap)
	labels, details := 0, 0
	for _, i := range c.shown {
		labels = max(labels, utf8.RuneCountInString(c.items[i].Label))
		details = max(details, min(utf8.RuneCountInString(c.items[i].Detail), completionMaxDetail))
	}
	w := m.labelX + labels*f.cw + m.padX
	if details > 0 {
		w += gtx.Dp(completionDetailGap) + details*f.cw
	}
	w = min(w, f.size.X-f.g.gutterW)
	h := min(completionRows, len(c.shown))*m.rowH + 2
	main := e.state.sel.Main()
	x, _ := e.coordsAt(c.from, 1)
	_, top := e.coordsAt(main.Head(), main.Assoc())
	x += f.g.textX - e.view.scrollX() - m.labelX
	y := top - e.view.scroll.Y
	m.box = e.placePopup(gtx, image.Rect(x, y, x+1, y+f.ch), image.Pt(w, h), false)
	return m
}

func (e *Editor) paintCompletionRow(gtx layout.Context, m menuLayout, row int, it CompletionItem, typed string, chosen bool) {
	p, f := e.Theme.Popup, e.f
	r := image.Rect(m.box.Min.X+1, m.box.Min.Y+1+row*m.rowH, m.box.Max.X-1, m.box.Min.Y+1+(row+1)*m.rowH)
	fg := p.Foreground
	if chosen {
		paint.FillShape(gtx.Ops, p.Selected, clip.UniformRRect(r, gtx.Dp(popupRadius)).Op(gtx.Ops))
		fg = p.SelectedForeground
	}
	textY := r.Min.Y + (m.rowH-f.ch)/2
	e.paintBadge(gtx, it.Kind, image.Pt(r.Min.X+m.padX, r.Min.Y+(m.rowH-m.badge)/2), m.badge)
	bold := 0
	if len(typed) <= len(it.Label) && strings.EqualFold(it.Label[:len(typed)], typed) {
		bold = len(typed)
	}
	label := []textSpan{{text: it.Label[:bold], color: fg, bold: true}, {text: it.Label[bold:], color: fg}}
	e.view.paintTextSpans(gtx, label, image.Pt(m.box.Min.X+m.labelX, textY), r)
	if it.Detail == "" {
		return
	}
	detail := truncate(it.Detail, completionMaxDetail)
	w := utf8.RuneCountInString(detail) * f.cw
	at := image.Pt(r.Max.X-m.padX-w, textY)
	labelEnd := m.box.Min.X + m.labelX + utf8.RuneCountInString(it.Label)*f.cw + gtx.Dp(completionDetailGap)/2
	if at.X < labelEnd {
		return
	}
	muted := p.Muted
	if chosen {
		muted = over(percent(fg, cmMutedOpacity), p.Selected)
	}
	e.view.paintTextSpans(gtx, []textSpan{{text: detail, color: muted}}, at, r)
}

// paintBadge draws kind's badge, a square of size with the kind's first
// letter, at at.
func (e *Editor) paintBadge(gtx layout.Context, kind CompletionKind, at image.Point, size int) {
	if kind == "" {
		return
	}
	r := image.Rectangle{Min: at, Max: at.Add(image.Pt(size, size))}
	paint.FillShape(gtx.Ops, e.badgeColor(kind), clip.UniformRRect(r, gtx.Dp(popupRadius)/2).Op(gtx.Ops))
	letter := textSpan{text: string(kind[:1]), color: e.Theme.Popup.Background, bold: true}
	img := e.view.spanImage(letter)
	e.view.paintImage(gtx, img, image.Pt(at.X+(size-img.size.X)/2, at.Y+(size-img.size.Y)/2), r)
}

// truncate is s cut to cols characters, an ellipsis ending it when cut.
func truncate(s string, cols int) string {
	if utf8.RuneCountInString(s) <= cols {
		return s
	}
	return string([]rune(s)[:cols-1]) + "…"
}

// paintCompletionDoc draws the chosen item's detail and documentation in a
// box beside the menu at menu: on its right, or on its left when there is
// no room on the right; nowhere when there is room on neither.
func (e *Editor) paintCompletionDoc(gtx layout.Context, menu image.Rectangle) {
	c := &e.completion
	it := c.items[c.shown[c.sel]]
	cols := min(completionDocCols, e.popupCols(gtx))
	var rows []popupRow
	code := e.Theme.tagColor(tags.Function(tags.VariableName))
	for _, l := range wrapText(it.Detail, cols) {
		if l != "" {
			rows = append(rows, popupRow{spans: []textSpan{{text: l, color: code}}})
		}
	}
	if doc := strings.TrimSpace(it.Documentation); doc != "" {
		rows = append(rows, textRows(doc, cols, e.Theme.Popup.Muted)...)
	}
	if it.Documentation == "" || len(rows) == 0 {
		return
	}
	rows = rows[:min(len(rows), popupMaxRows)]
	size := e.popupSize(gtx, rows)
	gap := gtx.Dp(popupGap)
	r := image.Rectangle{Min: image.Pt(menu.Max.X+gap, menu.Min.Y), Max: image.Pt(menu.Max.X+gap+size.X, menu.Min.Y+size.Y)}
	if r.Max.X > e.f.size.X {
		r = r.Sub(image.Pt(r.Min.X-(menu.Min.X-gap-size.X), 0))
	}
	if r.Min.X < e.f.g.gutterW {
		return
	}
	e.paintTextPopup(gtx, rows, r)
}
