// Package godemirror is a read-only code view for Gio: a monospace document
// with a line-number gutter, wrapping or horizontal scrolling, drawn as one
// image per visible row so a full screen of code costs a handful of quads.
//
// A View shows one document and a Split shows two side by side. Build them
// with New or NewSplit, which resolve a font family through fontconfig, or
// with NewFromFonts, which takes the font files themselves. Give them a
// Content and call Layout every frame.
//
// A View or a Split belongs to the goroutine that lays it out: call its
// methods and set its fields there, between frames. Only highlighting runs on
// a goroutine of its own, and its result is taken in by a later Layout.
//
// The view's deliberate limits, and the traps they set for a caller, are in
// docs/ceilings.md.
package godemirror

import (
	"image"
	"image/draw"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/glyphs"
)

// highlightPoll is how soon a frame looks again for highlighting still being
// worked out: about one frame at 60Hz.
const highlightPoll = 16 * time.Millisecond

// View shows a document. Its zero value is not usable; build one with New or
// NewFromFonts.
type View struct {
	// Wrap breaks lines at the view's width instead of scrolling sideways.
	Wrap bool
	// Theme colors the view; it starts as DarkTheme. A change shows on the
	// next Layout, which composes the rows on screen again.
	Theme Theme

	// r may be shared with the other side of a Split; px and composedIn are
	// the size and the theme this view's row images were composed at.
	r          *glyphs.Rasterizer
	size       unit.Sp
	px         float64
	composedIn Theme
	doc        *document
	longest    int
	// content is what SetContent or UpdateContent last put on screen, and
	// pending the highlighting still being worked out for it or for its
	// successor.
	content Content
	pending *pending

	// wraps and starts are wrapLine and rowStarts for wrapCols columns;
	// wrapCols is 0 when unwrapped and -1 when they must be rebuilt.
	wraps  [][]int
	starts []int
	// tops is lineTops: where each line starts once blocks are counted in.
	tops []int
	// blocks are this frame's laid-out BlockWidgets by the line they open
	// under, and extra their height per line.
	blocks   map[int][]laidBlock
	extra    []int
	wrapCols int
	scroll   image.Point
	// target is a ScrollToLine waiting for the next Layout.
	target *scrollTarget

	sel   selection
	press press
	// drag scrolls the view while a selection is held beyond its edge.
	drag dragScroll
	// matcher is the search set with SetSearch, and matches what it found in
	// the document on screen, in document order.
	matcher *matcher
	matches []Match
	// hover is the document line under the pointer, -1 for none, and hot the
	// hover group it belongs to (LineDecoration.HoverGroup), 0 for none.
	hover int
	hot   int
	// widgetAreas are where last frame's widgets were drawn, which a press
	// leaves to them rather than starting a selection.
	widgetAreas []image.Rectangle

	// labelWidth is the widest gutter label in cells.
	labelWidth int

	frameNo uint64
	hatches map[hatchKey]*rowImage
	rows    map[rowKey]*rowImage
	gutter  map[int]*rowImage
}

// rowKey names the cells a row image holds: a segment of line, and, unwrapped,
// span cells from col. span changes with the view's width, which a view
// opened small and then grown would otherwise go on showing cut.
type rowKey struct{ line, segment, col, span int }

type rowImage struct {
	op   paint.ImageOp
	size image.Point
	used uint64
}

// New builds a view drawing family at size. family is a fontconfig name
// (resolved with fc-match, as in "JetBrains Mono" or "monospace"), and a
// character it lacks is drawn from whichever installed font covers it. New
// fails with ErrNoFontconfig when fontconfig is not installed, and with
// another error when a font file it names cannot be read.
func New(family string, size unit.Sp) (*View, error) {
	r, err := glyphs.New(family)
	if err != nil {
		return nil, err
	}
	return newView(r, size), nil
}

func newView(r *glyphs.Rasterizer, size unit.Sp) *View {
	v := &View{Theme: DarkTheme, r: r, size: size, hover: -1}
	v.SetContent(Content{})
	return v
}

func (v *View) dropRows() {
	v.rows = map[rowKey]*rowImage{}
	v.gutter = map[int]*rowImage{}
	v.hatches = map[hatchKey]*rowImage{}
}

// Layout draws the view into the whole of gtx.Constraints.Max, takes the
// pointer and key events addressed to it (scroll, select, copy), and lays out
// each LineWidget on screen and every BlockWidget exactly once.
//
// While a selection is dragged beyond the view's top or bottom edge, or, when
// unwrapped, its left or right, Layout scrolls the view by the time since its
// last frame (gtx.Now) and asks for the next frame, faster the farther the
// pointer is beyond the edge. It scrolls no more at the document's ends, on
// release, or once the pointer is back inside.
//
// An error means nothing was drawn this frame: either the fonts could not be
// scaled to the size gtx asks for, or the content last set could not be
// highlighted. In the second case the error is reported once; that content
// stays plain after SetContent, and is never swapped in after UpdateContent.
func (v *View) Layout(gtx layout.Context) (layout.Dimensions, error) {
	f, err := v.prepare(gtx)
	if err != nil {
		return layout.Dimensions{}, err
	}
	v.starts = rowStarts(v.wraps, nil)
	v.tops = lineTops(v.starts, f.ch, v.extra)
	v.scrollToTarget(gtx, f)
	v.handlePointer(gtx, f)
	v.handleKeys(gtx)
	return v.paint(gtx, f), nil
}

// frame is one frame's measurements, carried between the phases of a layout.
type frame struct {
	size   image.Point
	g      geometry
	cw, ch int
}

// prepare takes in what changed since the last frame (scale, highlighting,
// width), leaves wraps current and lays the blocks out. The rows' starts and
// tops are the caller's to set, since a split view pads each side to the
// other's wrapping and blocks.
func (v *View) prepare(gtx layout.Context) (frame, error) {
	px := float64(gtx.Sp(v.size))
	if err := v.r.SetSize(px); err != nil {
		return frame{}, err
	}
	if px != v.px {
		v.px = px
		v.dropRows()
		v.wrapCols = -1
	}
	if v.Theme != v.composedIn {
		v.composedIn = v.Theme
		v.dropRows()
	}
	if err := v.takeHighlight(gtx); err != nil {
		return frame{}, err
	}
	v.frameNo++
	f := frame{size: gtx.Constraints.Max, cw: v.r.CellW(), ch: v.r.CellH()}
	f.g = v.geometry(f.size, f.cw)
	v.wrapRows(f.g.cols)
	v.layoutBlocks(gtx, f)
	return f, nil
}

func (v *View) paint(gtx layout.Context, f frame) layout.Dimensions {
	defer clip.Rect(image.Rectangle{Max: f.size}).Push(gtx.Ops).Pop()
	// The view's own handler goes first, beneath everything drawn after it:
	// a line widget laid out on top must be what a press over it reaches.
	event.Op(gtx.Ops, v)
	cursor := clip.Rect(image.Rect(f.g.gutterW, 0, f.size.X, f.size.Y)).Push(gtx.Ops)
	pointer.CursorText.Add(gtx.Ops)
	cursor.Pop()
	paint.Fill(gtx.Ops, v.Theme.Background)
	v.paintRows(gtx, f)
	v.evict()
	return layout.Dimensions{Size: f.size}
}

// geometry is where the gutter ends and how many cells of code fit beside it.
type geometry struct {
	gutterW int
	textX   int
	cols    int
}

func (v *View) geometry(size image.Point, cw int) geometry {
	gutterW := (v.labelWidth + 2) * cw
	textX := gutterW + cw/2
	return geometry{gutterW: gutterW, textX: textX, cols: max((size.X-textX)/cw, 1)}
}

func (v *View) wrapRows(cols int) {
	want := 0
	if v.Wrap {
		want = cols
	}
	if want == v.wrapCols {
		return
	}
	v.wraps = make([][]int, len(v.doc.lines))
	for i, l := range v.doc.lines {
		v.wraps[i] = wrapLine(l, want)
	}
	v.wrapCols = want
	// A row image is keyed by its line, segment and first column, which
	// name different cells once the wrapping changes.
	v.rows = map[rowKey]*rowImage{}
}

// visibleRow is one visual row on screen: a segment of a document line, at y.
type visibleRow struct{ line, segment, y int }

func (v *View) paintRows(gtx layout.Context, f frame) {
	size, g, cw, ch := f.size, f.g, f.cw, f.ch
	rows, blocks := v.visible(f)
	code := image.Rect(g.gutterW, 0, size.X, size.Y)
	v.paintLineBackgrounds(gtx, rows, code, ch)
	v.paintSpans(gtx, f, rows, code)
	v.paintSelection(gtx, f, rows, code)
	// Unwrapped rows are composed a page of columns at a time, two screens
	// wide, so scrolling sideways redraws a row only when it crosses a page.
	firstCol, x := 0, g.textX
	if !v.Wrap {
		firstCol = v.scroll.X / cw / g.cols * g.cols
		x -= v.scroll.X - firstCol*cw
	}
	for _, r := range rows {
		img := v.row(r.line, r.segment, firstCol, 2*g.cols)
		v.paintImage(gtx, img, image.Pt(x, r.y), code)
	}
	v.paintWidgets(gtx, f, rows, code)
	v.paintBlocks(gtx, blocks, code)
	// The gutter is painted over the code, so a row scrolled sideways slides
	// under it instead of showing through.
	paint.FillShape(gtx.Ops, v.Theme.GutterBackground, clip.Rect(image.Rect(0, 0, g.gutterW, size.Y)).Op())
	v.paintGutterDecorations(gtx, rows, g.gutterW, ch)
	for _, r := range rows {
		if r.segment > 0 {
			continue
		}
		v.paintGutterLabel(gtx, r.line, r.y, g, cw)
	}
}

// paintImage draws img at at, clipped to the area it may show in.
func (v *View) paintImage(gtx layout.Context, img *rowImage, at image.Point, area image.Rectangle) {
	if img.size.X == 0 {
		return
	}
	defer clip.Rect(area).Push(gtx.Ops).Pop()
	defer op.Offset(at).Push(gtx.Ops).Pop()
	defer clip.Rect(image.Rectangle{Max: img.size}).Push(gtx.Ops).Pop()
	img.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func (v *View) paintGutterLabel(gtx layout.Context, line, y int, g geometry, cw int) {
	img, ok := v.gutter[line]
	if !ok {
		img = v.compose([]rune(v.label(line)), nil)
		v.gutter[line] = img
	}
	img.used = v.frameNo
	at := image.Pt(g.gutterW-cw-img.size.X, y)
	v.paintImage(gtx, img, at, image.Rect(0, y, g.gutterW, y+img.size.Y))
}

// row returns the image of one visual row: a wrapped segment of line, or, for
// an unwrapped view, cols cells from firstCol.
func (v *View) row(line, segment, firstCol, cols int) *rowImage {
	k := rowKey{line: line, segment: segment, col: firstCol, span: cols}
	if img, ok := v.rows[k]; ok {
		img.used = v.frameNo
		return img
	}
	if segment >= len(v.wraps[line]) {
		return &rowImage{}
	}
	l, classes := v.doc.lines[line], v.doc.classes[line]
	from, to := firstCol, firstCol+cols
	if v.Wrap {
		from, to = v.wraps[line][segment], len(l)
		if next := segment + 1; next < len(v.wraps[line]) {
			to = v.wraps[line][next]
		}
	}
	from, to = min(from, len(l)), min(to, len(l))
	img := v.compose(l[from:to], classes[from:to])
	img.used = v.frameNo
	v.rows[k] = img
	return img
}

// compose rasterizes runes into a transparent row image, one cell each, styled
// by their classes; nil classes draws them in the gutter's color.
func (v *View) compose(runes []rune, classes []TokenClass) *rowImage {
	cw, ch := v.r.CellW(), v.r.CellH()
	if len(runes) == 0 {
		return &rowImage{}
	}
	dst := image.NewRGBA(image.Rect(0, 0, len(runes)*cw, ch))
	for i, c := range runes {
		if c == ' ' {
			continue
		}
		style := TokenStyle{Color: v.Theme.GutterForeground}
		if classes != nil {
			style = v.Theme.Syntax[classes[i]]
		}
		m := v.r.Mask(glyphs.Key{R: c, Italic: style.Italic})
		if m.Img == nil {
			continue
		}
		at := m.Img.Bounds().Add(m.Off).Add(image.Pt(i*cw, 0))
		draw.DrawMask(dst, at, image.NewUniform(style.Color), image.Point{}, m.Img, image.Point{}, draw.Over)
	}
	o := paint.NewImageOp(dst)
	o.Filter = paint.FilterNearest
	return &rowImage{op: o, size: dst.Bounds().Size()}
}

// evict drops the row images the last frame did not draw, so memory follows
// the viewport rather than everything ever scrolled past.
func (v *View) evict() {
	for k, img := range v.rows {
		if img.used != v.frameNo {
			delete(v.rows, k)
		}
	}
	for k, img := range v.gutter {
		if img.used != v.frameNo {
			delete(v.gutter, k)
		}
	}
	for k, img := range v.hatches {
		if img.used != v.frameNo {
			delete(v.hatches, k)
		}
	}
}
