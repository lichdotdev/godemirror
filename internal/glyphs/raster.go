// Package glyphs rasterizes a monospace font family into per-character
// coverage masks, for Gio views that compose text into images on the CPU
// instead of drawing Gio text paths.
package glyphs

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"os/exec"
	"slices"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// ErrNoFontconfig is what New fails with when fontconfig's fc-match is not
// installed.
var ErrNoFontconfig = errors.New("fontconfig not found: fc-match is not on PATH")

// fontStyles are the fontconfig styles of a family, indexed by styleIndex.
var fontStyles = [4]string{"regular", "bold", "italic", "bold:italic"}

func styleIndex(k Key) int {
	i := 0
	if k.Bold {
		i |= 1
	}
	if k.Italic {
		i |= 2
	}
	return i
}

// Key names one glyph: a character in one of the family's four styles.
type Key struct {
	R            rune
	Bold, Italic bool
}

type fontFace struct {
	sf   *sfnt.Font
	face font.Face
}

// Mask is a glyph's coverage, placed relative to its cell's top-left.
// Img is nil for a glyph that draws nothing.
type Mask struct {
	Img *image.Alpha
	Off image.Point
}

// Rasterizer turns characters into coverage masks on the CPU, once each, for
// the renderers that keep glyphs as images instead of Gio text paths. Built by
// New, it also owns the fallback Gio lacks: a character the family does not
// cover is drawn from whichever installed font fontconfig says covers it. A
// character nothing covers, or whose lookup fails, is drawn as a box (tofu).
type Rasterizer struct {
	px     float64
	styles [4]*fontFace
	byFile map[string]*fontFace
	byRune map[rune]*fontFace
	masks  map[Key]*Mask
	buf    sfnt.Buffer
	cellW  int
	cellH  int
	ascent int

	underline, strikethrough Rule
	// fcLookup names a font file covering a character; nil looks nothing up.
	fcLookup func(r rune) (string, error)
}

// New loads the four styles fontconfig resolves for family. Call SetSize
// before Mask or Cell. It fails with ErrNoFontconfig when fc-match is not
// installed.
func New(family string) (*Rasterizer, error) {
	r := &Rasterizer{byFile: map[string]*fontFace{}, fcLookup: fontconfigCovering}
	for i, style := range fontStyles {
		path, err := fcMatch(family + ":" + style)
		if err != nil {
			return nil, err
		}
		if r.styles[i], err = r.load(path); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// NewFromData loads the four styles from font files' contents, in
// styleIndex order (regular, bold, italic, bold italic). It never looks a
// fallback up: a character the fonts do not cover is drawn as tofu.
func NewFromData(styles [4][]byte) (*Rasterizer, error) {
	r := &Rasterizer{byFile: map[string]*fontFace{}}
	for i, data := range styles {
		f, err := parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s font: %w", fontStyles[i], err)
		}
		r.styles[i] = f
		// byFile holds every face SetSize sizes; no path can be "data:N".
		r.byFile[fmt.Sprint("data:", i)] = f
	}
	return r, nil
}

func fcMatch(pattern string) (string, error) {
	out, err := exec.Command("fc-match", "-f", "%{file}", pattern).Output()
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("%w (resolving %s)", ErrNoFontconfig, pattern)
	}
	if err != nil {
		return "", fmt.Errorf("fc-match %s: %w", pattern, err)
	}
	return string(out), nil
}

// fontconfigCovering names a font file that has a glyph for r.
func fontconfigCovering(r rune) (string, error) {
	charset := fmt.Sprintf(":charset=%x", r)
	out, err := exec.Command("fc-list", charset, "file").Output()
	if err != nil {
		return "", fmt.Errorf("fc-list %s: %w", charset, err)
	}
	covering := parseFcListFiles(string(out))
	if len(covering) == 0 {
		return "", nil
	}
	// fc-list knows who covers the character but in no useful order; fc-match
	// ranks every font against a plain monospace, so the first covering font
	// in its ranking is the closest regular face.
	ranked, err := exec.Command("fc-match", "-s", "-f", "%{file}\n", "monospace").Output()
	if err != nil {
		return "", fmt.Errorf("fc-match -s monospace: %w", err)
	}
	for f := range strings.Lines(string(ranked)) {
		if f = strings.TrimSpace(f); slices.Contains(covering, f) {
			return f, nil
		}
	}
	return covering[0], nil
}

// parseFcListFiles reads `fc-list <pattern> file`, one "path: " per line.
func parseFcListFiles(out string) []string {
	var files []string
	for line := range strings.Lines(out) {
		if f := strings.TrimSuffix(strings.TrimSpace(line), ":"); f != "" {
			files = append(files, f)
		}
	}
	return files
}

func (r *Rasterizer) load(path string) (*fontFace, error) {
	if f, ok := r.byFile[path]; ok {
		return f, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if r.px > 0 {
		if f.face, err = opentype.NewFace(f.sf, &opentype.FaceOptions{Size: r.px, DPI: 72}); err != nil {
			return nil, err
		}
	}
	r.byFile[path] = f
	return f, nil
}

// parse reads a font file, or the first font of a collection.
func parse(data []byte) (*fontFace, error) {
	coll, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	sf, err := coll.Font(0)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return &fontFace{sf: sf}, nil
}

// SetSize re-creates every face at px pixels per em, which drops every cached
// mask. Setting the size it already has does nothing.
func (r *Rasterizer) SetSize(px float64) error {
	if px == r.px {
		return nil
	}
	r.px = px
	for _, f := range r.byFile {
		face, err := opentype.NewFace(f.sf, &opentype.FaceOptions{Size: px, DPI: 72})
		if err != nil {
			return err
		}
		f.face = face
	}
	m := r.styles[0].face.Metrics()
	adv, _ := r.styles[0].face.GlyphAdvance('M')
	r.cellW = adv.Ceil()
	r.ascent = m.Ascent.Ceil()
	r.cellH = (m.Ascent + m.Descent).Ceil()
	r.underline, r.strikethrough = r.rules(m)
	r.masks = map[Key]*Mask{}
	r.byRune = map[rune]*fontFace{}
	return nil
}

func (r *Rasterizer) covers(f *fontFace, c rune) bool {
	gi, err := f.sf.GlyphIndex(&r.buf, c)
	return err == nil && gi != 0
}

// faceFor picks the family's face for k, or a fallback that covers its
// character, or nil when there is none.
func (r *Rasterizer) faceFor(k Key) *fontFace {
	primary := r.styles[styleIndex(k)]
	if r.covers(primary, k.R) {
		return primary
	}
	if f, ok := r.byRune[k.R]; ok {
		return f
	}
	f := r.fallback(k.R)
	r.byRune[k.R] = f
	return f
}

// fallback is an installed font covering c, or nil. A lookup or a font file
// that fails is no fallback: a missing glyph is not worth failing a frame
// over, and the character still shows, as tofu.
func (r *Rasterizer) fallback(c rune) *fontFace {
	if r.fcLookup == nil {
		return nil
	}
	path, err := r.fcLookup(c)
	if err != nil || path == "" {
		return nil
	}
	f, err := r.load(path)
	if err != nil || !r.covers(f, c) {
		return nil
	}
	return f
}

// Mask returns k's coverage, rasterizing it on first use.
func (r *Rasterizer) Mask(k Key) *Mask {
	if m, ok := r.masks[k]; ok {
		return m
	}
	m := r.tofu(Width(k.R))
	if f := r.faceFor(k); f != nil {
		m = &Mask{}
		dr, src, sp, _, ok := f.face.Glyph(fixed.P(0, r.ascent), k.R)
		if ok && !dr.Empty() {
			a := image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
			draw.Draw(a, a.Bounds(), src, sp, draw.Src)
			m.Img, m.Off = a, dr.Min
		}
	}
	r.masks[k] = m
	return m
}

// tofu is the box a character no font covers is drawn as, across the cells
// it takes. The fonts' own .notdef glyph is not used: in common monospace
// fonts (Go Mono, JetBrains Mono, Noto Sans Mono) it is empty, which would
// hide the character.
func (r *Rasterizer) tofu(cells int) *Mask {
	box := image.Rect(0, 0, cells*r.cellW, r.cellH).Inset(max(r.cellW/8, 1))
	a := image.NewAlpha(image.Rect(0, 0, box.Dx(), box.Dy()))
	stroke := max(r.cellW/12, 1)
	inner := a.Bounds().Inset(stroke)
	for y := range box.Dy() {
		for x := range box.Dx() {
			if !image.Pt(x, y).In(inner) {
				a.SetAlpha(x, y, color.Alpha{A: 0xff})
			}
		}
	}
	return &Mask{Img: a, Off: box.Min}
}

// Rule is a line across a cell, as text decorations draw it: Y is its top in
// pixels from the cell's top.
type Rule struct {
	Y, Thickness int
}

// rules are the underline and the strikethrough at the current size. The
// underline comes from the font's post table, as browsers take it; the
// strikethrough sits across the middle of the x-height with the same
// thickness, since sfnt does not read the OS/2 table that names it.
func (r *Rasterizer) rules(m font.Metrics) (underline, strikethrough Rule) {
	sf := r.styles[0].sf
	thickness, below := 1, 1
	if post := sf.PostTable(); post != nil && sf.UnitsPerEm() > 0 {
		scale := r.px / float64(sf.UnitsPerEm())
		thickness = max(int(float64(post.UnderlineThickness)*scale+0.5), 1)
		below = max(int(float64(-post.UnderlinePosition)*scale+0.5), 1)
	}
	underline = Rule{Y: min(r.ascent+below, r.cellH-thickness), Thickness: thickness}
	strikethrough = Rule{Y: r.ascent - m.XHeight.Round()/2 - thickness/2, Thickness: thickness}
	return underline, strikethrough
}

// Underline is where an underline is drawn at the current size.
func (r *Rasterizer) Underline() Rule { return r.underline }

// Strikethrough is where a line-through is drawn at the current size.
func (r *Rasterizer) Strikethrough() Rule { return r.strikethrough }

// CellW is the width of one monospace cell in pixels at the current size.
func (r *Rasterizer) CellW() int { return r.cellW }

// CellH is the height of one line in pixels at the current size.
func (r *Rasterizer) CellH() int { return r.cellH }
