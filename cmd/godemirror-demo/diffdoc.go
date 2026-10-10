package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/lichdotdev/godemirror"
)

// The diff laid out the way lich's web diff lays it out
// (frontend/src/lib/git/diff.ts, split-doc.ts, diff-blocks.ts): code lines
// without their +/-/space prefix, a separator line before every hunk that does
// not start the file, the unchanged run behind a separator offered to be
// pulled in, and every run of changes a block that can be reverted.

type lineKind int

const (
	lineContext lineKind = iota
	lineAdd
	lineDel
	lineSeparator
	lineFiller
)

type diffLine struct {
	kind             lineKind
	text             string
	oldLine, newLine int
	// block is the run of changes the line belongs to, 0 for none.
	block int
	// gap is the unchanged run a separator stands for, when it can be pulled in.
	gap *gap
	// id names the line across rebuilds, for a comment to stay on it: a hunk
	// line's place in the diff, or minus the new-file line of one pulled out
	// of a gap. Separators and fillers have none.
	id int
}

type hunk struct {
	oldStart, oldCount, newStart, newCount int
	lines                                  []diffLine
}

type fileDiff struct {
	path  string
	hunks []hunk
}

// gap is the unchanged run git left out before a hunk, in new-file lines
// [from, to], with oldFrom the old-file line pairing with from. key is from,
// which still names the gap once lines are pulled into others.
type gap struct{ key, from, to, oldFrom int }

// gapBefore measures the run between two hunks, or above the first one with
// prev nil. It is nil when there is nothing to pull in, or when old and new
// disagree on the run's length and it cannot be numbered.
func gapBefore(prev *hunk, h hunk) *gap {
	from, oldFrom := 1, 1
	if prev != nil {
		from, oldFrom = prev.newStart+prev.newCount, prev.oldStart+prev.oldCount
	}
	to := h.newStart - 1
	if to < from || h.oldStart-oldFrom != h.newStart-from {
		return nil
	}
	return &gap{key: from, from: from, to: to, oldFrom: oldFrom}
}

// parseFileDiff reads the first file of a unified diff.
func parseFileDiff(patch string) (fileDiff, error) {
	var f fileDiff
	var cur *hunk
	oldLine, newLine, id := 0, 0, 0
	for line := range strings.Lines(patch) {
		line = strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			if f.path != "" {
				return f, nil
			}
		case cur == nil && strings.HasPrefix(line, "+++ "):
			f.path = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "@@"):
			h, err := parseHunkHeader(line)
			if err != nil {
				return fileDiff{}, err
			}
			f.hunks = append(f.hunks, h)
			cur = &f.hunks[len(f.hunks)-1]
			oldLine, newLine = h.oldStart, h.newStart
		case cur == nil, strings.HasPrefix(line, `\`):
		default:
			id++
			cur.lines = append(cur.lines, hunkLine(line, id, &oldLine, &newLine))
		}
	}
	if f.path == "" {
		return fileDiff{}, fmt.Errorf("no file in the diff")
	}
	return f, nil
}

// hunkLine reads one line of a hunk, advancing the old and new line counters
// past it.
func hunkLine(line string, id int, oldLine, newLine *int) diffLine {
	switch {
	case strings.HasPrefix(line, "+"):
		l := diffLine{kind: lineAdd, text: line[1:], newLine: *newLine, id: id}
		*newLine++
		return l
	case strings.HasPrefix(line, "-"):
		l := diffLine{kind: lineDel, text: line[1:], oldLine: *oldLine, id: id}
		*oldLine++
		return l
	}
	l := diffLine{kind: lineContext, text: strings.TrimPrefix(line, " "), oldLine: *oldLine, newLine: *newLine, id: id}
	*oldLine++
	*newLine++
	return l
}

// parseHunkHeader reads "@@ -a,b +c,d @@", where a missing count is 1.
func parseHunkHeader(header string) (hunk, error) {
	h := hunk{oldCount: 1, newCount: 1}
	old, new, ok := strings.Cut(strings.TrimPrefix(header, "@@ -"), " +")
	if !ok {
		return hunk{}, fmt.Errorf("hunk header %q", header)
	}
	new, _, _ = strings.Cut(new, " ")
	if err := parseRange(old, &h.oldStart, &h.oldCount); err != nil {
		return hunk{}, fmt.Errorf("hunk header %q: %w", header, err)
	}
	if err := parseRange(new, &h.newStart, &h.newCount); err != nil {
		return hunk{}, fmt.Errorf("hunk header %q: %w", header, err)
	}
	return h, nil
}

func parseRange(r string, start, count *int) error {
	s, c, hasCount := strings.Cut(r, ",")
	var err error
	if *start, err = strconv.Atoi(s); err != nil {
		return err
	}
	if hasCount {
		*count, err = strconv.Atoi(c)
	}
	return err
}

// diffModel is the diff as currently shown: which gaps have been pulled in
// (from newText, the new file's lines) and which blocks reverted.
type diffModel struct {
	file     fileDiff
	newText  []string
	expanded map[int]bool
	reverted map[int]bool
}

// lines lays the model out as one column. A reverted block reads as the old
// text it went back to, and the new-file numbers below it move with it.
func (m *diffModel) lines() []diffLine {
	var out []diffLine
	shift, block := 0, 0
	for i, h := range m.file.hunks {
		var prev *hunk
		if i > 0 {
			prev = &m.file.hunks[i-1]
		}
		g := gapBefore(prev, h)
		switch {
		case g != nil && m.expanded[g.key] && m.newText != nil:
			for n := g.from; n <= g.to; n++ {
				out = append(out, diffLine{kind: lineContext, text: m.newText[n-1], oldLine: g.oldFrom + n - g.from, newLine: n + shift, id: -n})
			}
		case g != nil && m.newText != nil:
			out = append(out, diffLine{kind: lineSeparator, gap: g})
		case g != nil || i > 0:
			out = append(out, diffLine{kind: lineSeparator})
		}
		var emitted []diffLine
		emitted, shift, block = m.hunkLines(h, shift, block)
		out = append(out, emitted...)
	}
	return out
}

// hunkLines lays out one hunk, numbering its new side afresh: shift is how
// far reverts above have moved the new file's lines, and block the last block
// number used. A reverted block's deletions are the new file's lines again,
// and its additions are gone.
func (m *diffModel) hunkLines(h hunk, shift, block int) ([]diffLine, int, int) {
	var out []diffLine
	next := h.newStart + shift
	inBlock := false
	for _, l := range h.lines {
		changed := l.kind == lineAdd || l.kind == lineDel
		if changed && !inBlock {
			block++
		}
		inBlock = changed
		if changed {
			l.block = block
		}
		if changed && m.reverted[block] {
			if l.kind == lineAdd {
				continue
			}
			l = diffLine{kind: lineContext, text: l.text, oldLine: l.oldLine, id: l.id}
		}
		if l.kind != lineDel {
			l.newLine = next
			next++
		}
		out = append(out, l)
	}
	return out, next - (h.newStart + h.newCount), block
}

// side is one document of the layout, with the lines that carry a button.
type side struct {
	text        string
	labels      []string
	decorations []godemirror.LineDecoration
	// gaps and reverts place buttons: a separator's gap, a block's first line.
	gaps    map[int]*gap
	reverts map[int]int
	// ids is each line's diffLine.id.
	ids []int
}

// Colors of lich's dark diff rules, with their color-mix percentages as alpha
// (frontend/src/index.css): .diff-add/.diff-del at rest, .diff-block-hot under
// the pointer, .diff-gutter-*, .cm-diff-sep and .diff-filler.
var (
	emerald400 = color.NRGBA{R: 0x00, G: 0xd4, B: 0x92}
	emerald500 = color.NRGBA{R: 0x00, G: 0xbc, B: 0x7d}
	red400     = color.NRGBA{R: 0xff, G: 0x64, B: 0x67}
	red500     = color.NRGBA{R: 0xfb, G: 0x2c, B: 0x36}
	white      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff}

	decorationOf = map[lineKind]godemirror.LineDecoration{
		lineAdd: {
			Background: alpha(emerald400, 14), HoverBackground: alpha(emerald500, 20),
			GutterBackground: alpha(emerald500, 18), GutterStrip: alpha(emerald500, 100),
		},
		lineDel: {
			Background: alpha(red400, 12), HoverBackground: alpha(red500, 17),
			GutterBackground: alpha(red500, 15), GutterStrip: alpha(red500, 100),
		},
		lineSeparator: {Rule: alpha(white, 10)},
		lineFiller:    {Hatch: alpha(white, 7)},
	}
)

func alpha(c color.NRGBA, percent int) color.NRGBA {
	c.A = uint8(255 * percent / 100)
	return c
}

// unified is the single-column layout: deleted lines numbered from the old
// file, the rest from the new one.
func unified(lines []diffLine) side {
	return layoutSide(lines, true, func(l diffLine) int {
		if l.kind == lineDel {
			return l.oldLine
		}
		return l.newLine
	})
}

// split is the side-by-side layout: row N of one side is row N of the other.
// An unchanged line sits on both; a block pairs its deletions with its
// additions by position, and the shorter column is padded with fillers. The
// buttons live on the right, the new side.
func split(lines []diffLine) (left, right side) {
	var l, r []diffLine
	for i := 0; i < len(lines); {
		block := lines[i].block
		if block == 0 {
			l, r = append(l, lines[i]), append(r, lines[i])
			i++
			continue
		}
		var dels, adds []diffLine
		for ; i < len(lines) && lines[i].block == block; i++ {
			if lines[i].kind == lineDel {
				dels = append(dels, lines[i])
			} else {
				adds = append(adds, lines[i])
			}
		}
		for pair := range max(len(dels), len(adds)) {
			l = append(l, pairAt(dels, pair, block))
			r = append(r, pairAt(adds, pair, block))
		}
	}
	left = layoutSide(l, false, func(l diffLine) int { return l.oldLine })
	right = layoutSide(r, true, func(l diffLine) int { return l.newLine })
	return left, right
}

func pairAt(run []diffLine, i, block int) diffLine {
	if i < len(run) {
		return run[i]
	}
	return diffLine{kind: lineFiller, block: block}
}

// layoutSide builds one document; withButtons places the expand and revert
// buttons on it.
func layoutSide(lines []diffLine, withButtons bool, number func(diffLine) int) side {
	s := side{gaps: map[int]*gap{}, reverts: map[int]int{}}
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = l.text
		s.ids = append(s.ids, l.id)
		label := ""
		if n := number(l); n > 0 {
			label = strconv.Itoa(n)
		}
		s.labels = append(s.labels, label)
		d := decorationOf[l.kind]
		d.HoverGroup = l.block
		if withButtons && l.gap != nil {
			// The expander rides the separator, and its rule would run
			// through it (.cm-diff-sep.cm-diff-gap).
			d.Rule = color.NRGBA{}
			s.gaps[i] = l.gap
		}
		if _, seen := s.reverts[l.block]; withButtons && l.block != 0 && !seen {
			s.reverts[l.block] = i
		}
		s.decorations = append(s.decorations, d)
	}
	s.text = strings.Join(texts, "\n")
	return s
}
