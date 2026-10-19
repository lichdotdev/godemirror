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
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// ErrNoFontconfig is what fcMatch fails with when fontconfig's fc-match is not
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

// fontFace is one Rasterizer's view of a shared fontFile: its face at the
// rasterizer's size, which is not safe to share.
type fontFace struct {
	file *fontFile
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
// cover is drawn from whichever installed font covers it. A
// character nothing covers, or whose lookup fails, is drawn as a box (tofu).
type Rasterizer struct {
	px     float64
	styles [4]*fontFace
	byFile map[fontRef]*fontFace
	byRune map[rune]*fontFace
	masks  map[Key]*Mask
	buf    sfnt.Buffer
	cellW  int
	cellH  int
	ascent int

	underline, strikethrough Rule
	// lookup names a font covering a character, or none with an empty path;
	// nil looks nothing up.
	lookup func(r rune) (fontRef, error)
	// fallbacks are the faces lookup picked so far, in the order it picked
	// them, tried before asking it again: with fontconfig each ask is a
	// subprocess, and CJK text brings thousands of characters one face covers.
	fallbacks []*fontFace
}

// New loads the four styles of family. Where fontconfig is installed it
// resolves family as fc-match does; elsewhere family must be the name of an
// installed font (or "monospace") in the platform's font directories. Call
// SetSize before Mask or Cell. It fails with ErrFontNotFound when no font
// directory holds family.
func New(family string) (*Rasterizer, error) {
	refs, lookup, err := resolve(family)
	if err != nil {
		return nil, err
	}
	r := &Rasterizer{byFile: map[fontRef]*fontFace{}, lookup: lookup}
	for i, ref := range refs {
		if r.styles[i], err = r.load(ref); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// resolve names the fonts of family's four styles, in styleIndex order, and
// the lookup of a font covering a character the family lacks.
func resolve(family string) ([4]fontRef, func(rune) (fontRef, error), error) {
	if usesFontconfig {
		refs, err := fontconfigStyles(family)
		if !errors.Is(err, ErrNoFontconfig) {
			return refs, fontconfigLookup, err
		}
	}
	installed := installedFonts(fontDirs())
	refs, err := installed.family(family)
	return refs, installed.covering, err
}

func fontconfigStyles(family string) ([4]fontRef, error) {
	var refs [4]fontRef
	for i, style := range fontStyles {
		ref, err := fcMatch(family + ":" + style)
		if err != nil {
			return refs, err
		}
		refs[i] = ref
	}
	return refs, nil
}

func fontconfigLookup(c rune) (fontRef, error) {
	path, err := fontconfigCovering(c)
	return fontRef{path: path}, err
}

// NewFromData loads the four styles from font files' contents, in
// styleIndex order (regular, bold, italic, bold italic). It never looks a
// fallback up: a character the fonts do not cover is drawn as tofu.
func NewFromData(styles [4][]byte) (*Rasterizer, error) {
	r := &Rasterizer{byFile: map[fontRef]*fontFace{}}
	for i, data := range styles {
		file, err := parse(data, 0)
		if err != nil {
			return nil, fmt.Errorf("%s font: %w", fontStyles[i], err)
		}
		f := &fontFace{file: file}
		r.styles[i] = f
		// byFile holds every face SetSize sizes; no path can be "data:N".
		r.byFile[fontRef{path: fmt.Sprint("data:", i)}] = f
	}
	return r, nil
}

// fcMatch names the font fontconfig picks for pattern, the font's index first
// since a path may hold a colon.
func fcMatch(pattern string) (fontRef, error) {
	out, err := exec.Command("fc-match", "-f", "%{index}:%{file}", pattern).Output()
	if errors.Is(err, exec.ErrNotFound) {
		return fontRef{}, fmt.Errorf("%w (resolving %s)", ErrNoFontconfig, pattern)
	}
	if err != nil {
		return fontRef{}, fmt.Errorf("fc-match %s: %w", pattern, err)
	}
	index, path, ok := strings.Cut(string(out), ":")
	n, err := strconv.Atoi(index)
	if !ok || err != nil {
		return fontRef{}, fmt.Errorf("fc-match %s: want index:file, got %q", pattern, out)
	}
	return fontRef{path: path, index: n}, nil
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
	ranked, err := rankedAgainstMono()
	if err != nil {
		return "", err
	}
	for f := range strings.Lines(ranked) {
		if f = strings.TrimSpace(f); slices.Contains(covering, f) {
			return f, nil
		}
	}
	return covering[0], nil
}

// rankedAgainstMono is every installed font file ranked against a plain
// monospace, asked once per process: fc-list knows who covers a character but
// in no useful order, so the first covering font in this ranking is the
// closest regular face. A font installed since is missing from it and is
// picked only when it is the first fc-list names.
var rankedAgainstMono = sync.OnceValues(func() (string, error) {
	out, err := exec.Command("fc-match", "-s", "-f", "%{file}\n", "monospace").Output()
	if err != nil {
		return "", fmt.Errorf("fc-match -s monospace: %w", err)
	}
	return string(out), nil
})

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

func (r *Rasterizer) load(ref fontRef) (*fontFace, error) {
	if f, ok := r.byFile[ref]; ok {
		return f, nil
	}
	file, err := openFontFile(ref)
	if err != nil {
		return nil, err
	}
	f := &fontFace{file: file}
	if r.px > 0 {
		if f.face, err = opentype.NewFace(file.sf, &opentype.FaceOptions{Size: r.px, DPI: 72}); err != nil {
			return nil, err
		}
	}
	r.byFile[ref] = f
	return f, nil
}

// SetSize re-creates every face at px pixels per em, which drops every cached
// mask. Setting the size it already has does nothing.
func (r *Rasterizer) SetSize(px float64) error {
	if px == r.px {
		return nil
	}
	r.px = px
	for _, f := range r.byFile {
		face, err := opentype.NewFace(f.file.sf, &opentype.FaceOptions{Size: px, DPI: 72})
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
	gi, err := f.file.sf.GlyphIndex(&r.buf, c)
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
	if r.lookup == nil {
		return nil
	}
	for _, f := range r.fallbacks {
		if r.covers(f, c) {
			return f
		}
	}
	ref, err := r.lookup(c)
	if err != nil || ref.path == "" {
		return nil
	}
	f, err := r.load(ref)
	if err != nil || !r.covers(f, c) {
		return nil
	}
	r.fallbacks = append(r.fallbacks, f)
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
	sf := r.styles[0].file.sf
	thickness, below := 1, 1
	if post := sf.PostTable(); post != nil && sf.UnitsPerEm() > 0 {
		scale := r.px / float64(sf.UnitsPerEm())
		thickness = max(int(float64(post.UnderlineThickness)*scale+0.5), 1)
		below = max(int(float64(-post.UnderlinePosition)*scale+0.5), 1)
	}
	underline = Rule{Y: min(r.ascent+below, r.cellH-thickness), Thickness: thickness}
	strikethrough = Rule{Y: strikethroughY(r.ascent, m.XHeight, thickness), Thickness: thickness}
	return underline, strikethrough
}

// strikethroughY centers a line of thickness on the middle of the x-height.
// x/image gives some fonts' x-height negative (DejaVu Sans Mono's, measured
// from its glyph bounds), others' positive, so only its size is used.
func strikethroughY(ascent int, xHeight fixed.Int26_6, thickness int) int {
	return ascent - abs(xHeight.Round())/2 - thickness/2
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Underline is where an underline is drawn at the current size.
func (r *Rasterizer) Underline() Rule { return r.underline }

// Strikethrough is where a line-through is drawn at the current size.
func (r *Rasterizer) Strikethrough() Rule { return r.strikethrough }

// CellW is the width of one monospace cell in pixels at the current size.
func (r *Rasterizer) CellW() int { return r.cellW }

// CellH is the height of one line in pixels at the current size.
func (r *Rasterizer) CellH() int { return r.cellH }
