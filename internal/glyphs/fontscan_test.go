package glyphs

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
)

// fontDir is a fresh directory holding files, by name.
func fontDir(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAScannedFamilyTakesEachStyleFromItsOwnFile(t *testing.T) {
	dir := fontDir(t, map[string][]byte{
		"a.ttf": gomonobolditalic.TTF, "b.ttf": gomonoitalic.TTF,
		"c.ttf": gomonobold.TTF, "d.ttf": gomono.TTF, "notes.txt": []byte("not a font"),
	})
	refs, err := installedFonts([]string{dir}).family("GO MONO")
	if err != nil {
		t.Fatal(err)
	}
	want := [4]string{"d.ttf", "c.ttf", "b.ttf", "a.ttf"}
	for i, ref := range refs {
		if got := filepath.Base(ref.path); got != want[i] {
			t.Errorf("%s from %s, want %s", fontStyles[i], got, want[i])
		}
	}
}

func TestAStyleTheScannedFamilyLacksIsDrawnWithItsRegular(t *testing.T) {
	dir := fontDir(t, map[string][]byte{"bold.ttf": gomonobold.TTF, "regular.ttf": gomono.TTF})
	refs, err := installedFonts([]string{dir}).family("Go Mono")
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(refs[styleIndex(Key{Italic: true})].path); got != "regular.ttf" {
		t.Errorf("italic from %s, want regular.ttf", got)
	}
}

func TestAScannedFallbackIsTheFirstFontCoveringTheCharacter(t *testing.T) {
	idx := installedFonts([]string{fontDir(t, map[string][]byte{"regular.ttf": gomono.TTF})})
	if ref, _ := idx.covering('é'); filepath.Base(ref.path) != "regular.ttf" {
		t.Errorf("é covered by %q, want regular.ttf", ref.path)
	}
	if ref, _ := idx.covering(0x50000); ref.path != "" {
		t.Errorf("an unassigned character covered by %q", ref.path)
	}
}
