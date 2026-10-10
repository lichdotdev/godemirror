package godemirror

import (
	"image/color"
	"testing"
)

func TestOverMixesInSRGB(t *testing.T) {
	base := color.NRGBA{R: 0x09, G: 0x09, B: 0x0b, A: 0xff}
	cases := []struct {
		c, want color.NRGBA
	}{
		{color.NRGBA{R: 0xff, A: 0xff}, color.NRGBA{R: 0xff, A: 0xff}},
		{color.NRGBA{R: 0xff, A: 0}, base},
		{color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 128}, color.NRGBA{R: 0x84, G: 0x84, B: 0x85, A: 0xff}},
	}
	for _, c := range cases {
		if got := over(c.c, base); got != c.want {
			t.Errorf("over(%v) = %v, want %v", c.c, got, c.want)
		}
	}
}
