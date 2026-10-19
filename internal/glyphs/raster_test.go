package glyphs

import (
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/math/fixed"
)

func TestParseFcListFiles(t *testing.T) {
	out := "/usr/share/fonts/TTF/A.ttf: \n/usr/share/fonts/TTF/B Mono.ttf: \n"
	want := []string{"/usr/share/fonts/TTF/A.ttf", "/usr/share/fonts/TTF/B Mono.ttf"}
	if got := parseFcListFiles(out); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := parseFcListFiles(""); len(got) != 0 {
		t.Fatalf("empty output parsed as %q", got)
	}
}

func TestACharacterNoFontCoversIsDrawnAsABox(t *testing.T) {
	r, err := NewFromData([4][]byte{gomono.TTF, gomono.TTF, gomono.TTF, gomono.TTF})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetSize(24); err != nil {
		t.Fatal(err)
	}
	m := r.Mask(Key{R: 0x50000})
	if m.Img == nil {
		t.Fatal("an unassigned character draws nothing")
	}
	if b := m.Img.Bounds().Add(m.Off); !b.In(image.Rect(0, 0, r.CellW(), r.CellH())) {
		t.Fatalf("box %v spills out of its %dx%d cell", b, r.CellW(), r.CellH())
	}
}

// x/image reports DejaVu Sans Mono's x-height negative (-6.36 at 12px) and
// Noto Sans Mono's positive; either way the line-through must sit above the
// baseline, inside the x-height.
func TestTheStrikethroughSitsAboveTheBaselineWhateverTheXHeightSign(t *testing.T) {
	const ascent, thickness, xHeightPx = 12, 1, 6
	for _, xHeight := range []fixed.Int26_6{fixed.I(xHeightPx) + 23, -(fixed.I(xHeightPx) + 23)} {
		y := strikethroughY(ascent, xHeight, thickness)
		if y >= ascent || y <= ascent-xHeightPx-1 {
			t.Errorf("x-height %v: strikethrough at %d, want inside the x-height above the baseline at %d", xHeight, y, ascent)
		}
	}
}

// Each fallback lookup is a subprocess run inside a frame: a view of CJK text,
// thousands of distinct characters the family lacks, froze the window for
// seconds when every one of them asked again.
func TestALoadedFallbackServesTheCharactersItCovers(t *testing.T) {
	fallback, err := fcMatch("DejaVu Sans")
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewFromData([4][]byte{gomono.TTF, gomono.TTF, gomono.TTF, gomono.TTF})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetSize(24); err != nil {
		t.Fatal(err)
	}
	lookups := 0
	r.lookup = func(rune) (fontRef, error) { lookups++; return fontRef{path: fallback}, nil }
	const first, distinct = 0x474, 20 // Cyrillic Go Mono lacks and DejaVu Sans draws
	for c := rune(first); c < first+distinct; c++ {
		if f := r.faceFor(Key{R: c}); f == nil {
			t.Fatalf("%U found no face", c)
		}
	}
	if lookups != 1 {
		t.Fatalf("%d distinct characters looked a fallback up %d times, want 1", distinct, lookups)
	}
}

// Every view has a Rasterizer of its own, and a diff has a view per file; one
// read of each font file per view was tens of megabytes per file shown.
func TestRasterizersShareAFontFile(t *testing.T) {
	path, err := fcMatch("DejaVu Sans")
	if err != nil {
		t.Fatal(err)
	}
	var faces [2]*fontFace
	for i := range faces {
		r, err := NewFromData([4][]byte{gomono.TTF, gomono.TTF, gomono.TTF, gomono.TTF})
		if err != nil {
			t.Fatal(err)
		}
		if err := r.SetSize(24); err != nil {
			t.Fatal(err)
		}
		if faces[i], err = r.load(fontRef{path: path}); err != nil {
			t.Fatal(err)
		}
	}
	if faces[0].file != faces[1].file {
		t.Errorf("two rasterizers each read %s", path)
	}
	if faces[0].face == faces[1].face {
		t.Error("two rasterizers draw through one sized face, which is not safe to share")
	}
}

// The ranking against monospace is the same answer for every character, and
// asking for it again is another subprocess inside a frame.
func TestFallbackLookupsRankAgainstMonospaceOnce(t *testing.T) {
	if _, err := fontconfigCovering('A'); err != nil {
		t.Fatal(err)
	}
	calls := countCalls(t, "fc-match")
	path, err := fontconfigCovering('B')
	if err != nil || path == "" {
		t.Fatalf("fontconfigCovering('B') = %q, %v", path, err)
	}
	if got := calls(); got != 0 {
		t.Fatalf("a second lookup ran fc-match %d times, want 0", got)
	}
}

// countCalls puts a wrapper of the real command first on PATH and returns how
// many times it has run since.
func countCalls(t *testing.T, command string) func() int {
	t.Helper()
	real, err := exec.LookPath(command)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, command), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() int {
		out, err := os.ReadFile(log)
		if os.IsNotExist(err) {
			return 0
		}
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(string(out), "\n")
	}
}
