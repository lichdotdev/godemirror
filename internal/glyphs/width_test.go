package glyphs

import (
	"image"
	"testing"

	"golang.org/x/image/font/gofont/gomono"
)

func TestWidth(t *testing.T) {
	cases := []struct {
		r    rune
		want int
	}{
		{'a', 1},
		{'é', 1},
		{'ｱ', 1}, // halfwidth katakana
		{'中', 2},
		{'ア', 2},
		{'Ａ', 2}, // fullwidth Latin
		{'😀', 2},
		{'́', 1},     // combining acute: zero width, out of the table
		{0x2A6DF, 2}, // CJK Extension B
		{0x2FFFD, 2}, // unassigned in plane 2, wide by default
	}
	for _, c := range cases {
		if got := Width(c.r); got != c.want {
			t.Errorf("Width(%U) = %d, want %d", c.r, got, c.want)
		}
	}
}

func TestAWideCharacterNoFontCoversIsATwoCellBox(t *testing.T) {
	r, err := NewFromData([4][]byte{gomono.TTF, gomono.TTF, gomono.TTF, gomono.TTF})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetSize(24); err != nil {
		t.Fatal(err)
	}
	b := r.Mask(Key{R: '中'}).Img.Bounds().Add(r.Mask(Key{R: '中'}).Off)
	if !b.In(image.Rect(0, 0, 2*r.CellW(), r.CellH())) || b.Max.X <= r.CellW() {
		t.Fatalf("box %v, want one inside two %dx%d cells and wider than one", b, r.CellW(), r.CellH())
	}
}
