package godemirror

import (
	"slices"
	"testing"
)

func TestWrapLineBreaksAfterSpacesAndSplitsLongWords(t *testing.T) {
	cases := []struct {
		line string
		cols int
		want []int
	}{
		{"short", 10, []int{0}},
		{"aaa bbb ccc", 8, []int{0, 8}},
		// A continuation row is indented by hangOf, so it has less room.
		{"aaa bbbbbbbbbbbb", 6, []int{0, 4, 8, 12}},
		{"abcdefghij", 4, []int{0, 4, 6, 8}},
		{"    aaaa bbbb cccc", 12, []int{0, 9, 14}},
		{"            abcdefgh", 8, []int{0, 8, 12, 16}},
		// A word longer than the row after the indentation is split on the
		// first row rather than leaving it the indentation alone.
		{"    abcdefghij k", 10, []int{0, 10, 15}},
		{"abcdef", 1, []int{0, 1, 2, 3, 4, 5}},
		{"", 4, []int{0}},
		{"aaa bbb ccc", 0, []int{0}},
	}
	for _, c := range cases {
		if got := wrapLine([]rune(c.line), c.cols); !slices.Equal(got, c.want) {
			t.Fatalf("wrapLine(%q, %d) = %v, want %v", c.line, c.cols, got, c.want)
		}
	}
}

func TestHangOfIndentsByTheLinesOwnWhitespaceAndTwoCappedAtHalfARow(t *testing.T) {
	for _, c := range []struct {
		line string
		cols int
		want int
	}{
		{"text", 20, 2},
		{"    text", 20, 6},
		{"                    text", 20, 10},
		{"    text", 3, 1},
		{"    text", 1, 0},
		{"    text", 0, 0},
	} {
		if got := hangOf([]rune(c.line), c.cols); got != c.want {
			t.Errorf("hangOf(%q, %d) = %d, want %d", c.line, c.cols, got, c.want)
		}
	}
}

func TestRowStartsCountsEveryWrappedRow(t *testing.T) {
	wraps := [][]int{{0}, {0}, {0, 4, 8}, {0}}
	if got, want := rowStarts(wraps, nil), []int{0, 1, 2, 5, 6}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := rowStarts(wraps, []int{2, 1, 1}), []int{0, 2, 3, 6, 7}; !slices.Equal(got, want) {
		t.Fatalf("padded: got %v, want %v", got, want)
	}
}

func TestLineTopsAndLocateAccountForBlocks(t *testing.T) {
	starts := []int{0, 1, 3, 4} // line 1 wraps into two rows
	extra := []int{0, 25}       // 25px of blocks under line 1
	tops := lineTops(starts, 10, extra)
	if want := []int{0, 10, 55, 65}; !slices.Equal(tops, want) {
		t.Fatalf("tops = %v, want %v", tops, want)
	}
	cases := []struct {
		y, line, segment int
		ok               bool
	}{
		{0, 0, 0, true},
		{9, 0, 0, true},
		{10, 1, 0, true},
		{29, 1, 1, true},
		{30, 1, -1, true},
		{54, 1, -1, true},
		{55, 2, 0, true},
		{65, 0, 0, false},
		{-1, 0, 0, false},
	}
	for _, c := range cases {
		line, segment, ok := locate(tops, starts, 10, c.y)
		if line != c.line || segment != c.segment || ok != c.ok {
			t.Errorf("locate(%d) = %d %d %v, want %d %d %v", c.y, line, segment, ok, c.line, c.segment, c.ok)
		}
	}
}
