package glyphs

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
)

// A family's bold in a collection is a font of its own there: drawn from the
// collection's first font instead, bold text came out regular.
func TestFontconfigResolvesAStyleToItsFontInACollection(t *testing.T) {
	dir := t.TempDir()
	ttc := filepath.Join(dir, "GoMono.ttc")
	if err := os.WriteFile(ttc, collection(gomono.TTF, gomonobold.TTF), 0o644); err != nil {
		t.Fatal(err)
	}
	onlyFontconfigDir(t, dir)
	refs, err := fontconfigStyles("Go Mono")
	if err != nil {
		t.Fatal(err)
	}
	if want := (fontRef{path: ttc, index: 1}); refs[styleIndex(Key{Bold: true})] != want {
		t.Fatalf("bold = %+v, want %+v", refs[styleIndex(Key{Bold: true})], want)
	}
}

// A fallback in a collection is the font fontconfig ranks first there, not
// the file's first: a regular fallback after its bold was drawn bold.
func TestAFontconfigFallbackIsItsFontInACollection(t *testing.T) {
	dir := t.TempDir()
	ttc := filepath.Join(dir, "GoMono.ttc")
	if err := os.WriteFile(ttc, collection(gomonobold.TTF, gomono.TTF), 0o644); err != nil {
		t.Fatal(err)
	}
	onlyFontconfigDir(t, dir)
	ranked := rankedAgainstMono
	rankedAgainstMono = sync.OnceValues(rankAgainstMono)
	t.Cleanup(func() { rankedAgainstMono = ranked })
	ref, err := fontconfigCovering('é')
	if err != nil {
		t.Fatal(err)
	}
	if want := (fontRef{path: ttc, index: 1}); ref != want {
		t.Fatalf("é covered by %+v, want %+v", ref, want)
	}
}

// onlyFontconfigDir points fontconfig at dir alone for the rest of the test.
func onlyFontconfigDir(t *testing.T, dir string) {
	t.Helper()
	conf := filepath.Join(t.TempDir(), "fonts.conf")
	xml := `<?xml version="1.0"?><fontconfig><dir>` + dir + `</dir><cachedir>` + t.TempDir() + `</cachedir></fontconfig>`
	if err := os.WriteFile(conf, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FONTCONFIG_FILE", conf)
}

// collection is a TrueType collection of fonts, in order. A collection's table
// offsets count from the start of the file, so each font's are moved by
// where it lands.
func collection(fonts ...[]byte) []byte {
	const headerSize, recordSize, firstRecord = 12, 16, 12
	out := make([]byte, headerSize+4*len(fonts))
	copy(out, "ttcf")
	binary.BigEndian.PutUint32(out[4:], 0x00010000)
	binary.BigEndian.PutUint32(out[8:], uint32(len(fonts)))
	for i, font := range fonts {
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		at := uint32(len(out))
		binary.BigEndian.PutUint32(out[headerSize+4*i:], at)
		font = append([]byte(nil), font...)
		numTables := int(binary.BigEndian.Uint16(font[4:]))
		for r := range numTables {
			offset := font[firstRecord+r*recordSize+8:]
			binary.BigEndian.PutUint32(offset, binary.BigEndian.Uint32(offset)+at)
		}
		out = append(out, font...)
	}
	return out
}
