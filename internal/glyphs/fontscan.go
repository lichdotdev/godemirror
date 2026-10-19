package glyphs

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// ErrFontNotFound is what New fails with, where fontconfig is not installed,
// when no font directory holds the family it was asked for.
var ErrFontNotFound = errors.New("font family not found")

// installedFont is one font found in a font directory.
type installedFont struct {
	ref    fontRef
	family string // lower case
	style  int    // styleIndex, or -1 for a style outside the four
}

// fontIndex is every font in a set of font directories, the platform's
// monospace families first, then its fallback families, regular faces ahead
// of the other styles of each.
type fontIndex struct {
	fonts []installedFont

	mu sync.Mutex
	// coveringByRune is every answer covering gave, misses too: a miss reads
	// every font in the index.
	coveringByRune map[rune]fontRef
}

// fontIndexes are scanned once per process for each set of directories. A
// font installed since is not found until the process restarts.
var fontIndexes = struct {
	sync.Mutex
	byDirs map[string]*fontIndex
}{byDirs: map[string]*fontIndex{}}

func installedFonts(dirs []string) *fontIndex {
	key := strings.Join(dirs, string(os.PathListSeparator))
	fontIndexes.Lock()
	defer fontIndexes.Unlock()
	if idx, ok := fontIndexes.byDirs[key]; ok {
		return idx
	}
	idx := scanFonts(dirs)
	fontIndexes.byDirs[key] = idx
	return idx
}

// scanFonts keeps a font's names alone: keeping its parsed tables to answer
// covering faster held about 100 MB for 1,400 fonts.
//
// ponytail: every font file is read on each process's first New, 0.1 s for
// 1,400 fonts; a persistent on-disk index, as fontconfig keeps, if that ever
// shows.
func scanFonts(dirs []string) *fontIndex {
	var fonts []installedFont
	for _, dir := range dirs {
		// A directory that is missing or unreadable, and a file that is not a
		// font, are skipped: the fonts found elsewhere are still worth using.
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !isFontFile(d) {
				return nil
			}
			fonts = append(fonts, readFonts(path)...)
			return nil
		})
	}
	slices.SortStableFunc(fonts, func(a, b installedFont) int {
		return cmp.Or(cmp.Compare(familyRank(a.family), familyRank(b.family)), cmp.Compare(styleRank(a.style), styleRank(b.style)))
	})
	return &fontIndex{fonts: fonts, coveringByRune: map[rune]fontRef{}}
}

func isFontFile(d fs.DirEntry) bool {
	switch strings.ToLower(filepath.Ext(d.Name())) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return !d.IsDir()
	}
	return false
}

// readFonts is every font in the file at path, or none when it cannot be
// read as one.
func readFonts(path string) []installedFont {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	coll, err := sfnt.ParseCollectionReaderAt(f)
	if err != nil {
		return nil
	}
	var fonts []installedFont
	var buf sfnt.Buffer
	for i := range coll.NumFonts() {
		sf, err := coll.Font(i)
		if err != nil {
			continue
		}
		family := nameOf(sf, &buf, sfnt.NameIDTypographicFamily, sfnt.NameIDFamily)
		if family == "" {
			continue
		}
		fonts = append(fonts, installedFont{
			ref:    fontRef{path: path, index: i},
			family: strings.ToLower(family),
			style:  styleOf(nameOf(sf, &buf, sfnt.NameIDTypographicSubfamily, sfnt.NameIDSubfamily)),
		})
	}
	return fonts
}

// nameOf is the first of ids the font has a name for, or "".
func nameOf(sf *sfnt.Font, buf *sfnt.Buffer, ids ...sfnt.NameID) string {
	for _, id := range ids {
		if s, err := sf.Name(buf, id); err == nil && s != "" {
			return s
		}
	}
	return ""
}

func styleOf(subfamily string) int {
	switch strings.ToLower(subfamily) {
	case "regular", "book", "normal", "roman":
		return 0
	case "bold":
		return 1
	case "italic", "oblique":
		return 2
	case "bold italic", "bold oblique":
		return 3
	}
	return -1
}

// styleRank puts a family's regular face first and a style outside the four
// last.
func styleRank(style int) int {
	if style < 0 {
		return len(fontStyles)
	}
	return style
}

func familyRank(family string) int {
	preferred := slices.Concat(monospaceFamilies, fallbackFamilies)
	i := slices.IndexFunc(preferred, func(f string) bool { return strings.EqualFold(f, family) })
	if i < 0 {
		return len(preferred)
	}
	return i
}

// family names the fonts of family's four styles, as fontconfig would: a
// style the family lacks is drawn with its regular face. "monospace" is the
// first of the platform's monospace families that is installed.
func (idx *fontIndex) family(family string) ([4]fontRef, error) {
	candidates := []string{family}
	if strings.EqualFold(family, "monospace") {
		candidates = monospaceFamilies
	}
	for _, name := range candidates {
		if refs, ok := idx.styles(strings.ToLower(name)); ok {
			return refs, nil
		}
	}
	return [4]fontRef{}, fmt.Errorf("%w: %s", ErrFontNotFound, family)
}

// styles is family's fonts by styleIndex, the family's first face standing
// in for a style it lacks; ok is false when no font is of family.
func (idx *fontIndex) styles(family string) (refs [4]fontRef, ok bool) {
	var found [4]bool
	for _, f := range idx.fonts {
		if f.family != family {
			continue
		}
		if !ok {
			refs, ok = [4]fontRef{f.ref, f.ref, f.ref, f.ref}, true
		}
		if f.style >= 0 && !found[f.style] {
			refs[f.style], found[f.style] = f.ref, true
		}
	}
	return refs, ok
}

// covering names the first font in the index with a glyph for c, or none.
func (idx *fontIndex) covering(c rune) (fontRef, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if ref, ok := idx.coveringByRune[c]; ok {
		return ref, nil
	}
	var ref fontRef
	var buf sfnt.Buffer
	for _, f := range idx.fonts {
		if fontCovers(f.ref, c, &buf) {
			ref = f.ref
			break
		}
	}
	idx.coveringByRune[c] = ref
	return ref, nil
}

// fontCovers reads whether the font at ref has a glyph for c. A font that
// cannot be read covers nothing, as when the index was built.
func fontCovers(ref fontRef, c rune, buf *sfnt.Buffer) bool {
	f, err := os.Open(ref.path)
	if err != nil {
		return false
	}
	defer f.Close()
	coll, err := sfnt.ParseCollectionReaderAt(f)
	if err != nil {
		return false
	}
	sf, err := coll.Font(ref.index)
	if err != nil {
		return false
	}
	gi, err := sf.GlyphIndex(buf, c)
	return err == nil && gi != 0
}
