package glyphs

import "slices"

//go:generate go run mkwidth.go

// Width is the cells r takes in a monospace grid, as a terminal lays it out:
// 2 for East Asian Wide and Fullwidth characters and for emoji shown as
// emoji by default (Emoji_Presentation), 1 for everything else. Zero-width
// characters (combining marks, joiners, variation selectors) take 1 too.
// The table is Unicode 18.0.0's; mkwidth.go regenerates it.
func Width(r rune) int {
	_, found := slices.BinarySearchFunc(wide, r, func(rg [2]rune, r rune) int {
		switch {
		case rg[1] < r:
			return -1
		case rg[0] > r:
			return 1
		}
		return 0
	})
	if found {
		return 2
	}
	return 1
}
