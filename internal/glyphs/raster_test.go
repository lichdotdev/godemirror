package glyphs

import (
	"image"
	"slices"
	"testing"

	"golang.org/x/image/font/gofont/gomono"
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
