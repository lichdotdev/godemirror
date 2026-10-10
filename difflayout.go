package godemirror

import (
	"fmt"
	"strconv"
	"strings"
)

// Gap is an unchanged run of lines a diff leaves out: new-file lines From to
// To (1-based, inclusive), with OldFrom the old-file line that pairs with
// From. A DiffView hands one to DiffOptions.Expand and takes the lines back
// through FillGap.
type Gap struct {
	From, To, OldFrom int

	// key is where the gap started before anything was filled in, which
	// still names it after; gen is the content it belongs to.
	key, gen int
}

// Side is which file of a diff a line is in.
type Side uint8

const (
	// OldSide is the original file, the one a deletion is taken from.
	OldSide Side = iota
	// NewSide is the modified file, the one an addition is put in.
	NewSide
)

// ChangedLine is a deleted line (OldSide, numbered in the original file) or
// an added one (NewSide, numbered in the modified file), with its text.
type ChangedLine struct {
	Side Side
	// Line is 1-based.
	Line int
	Text string
}

// Chunk is a run of changed lines with no unchanged line between them. Each
// side's start is the 1-based number of its first line there; a side the
// chunk has no lines in (Count 0) starts at the line the chunk sits before.
type Chunk struct {
	OldStart, OldCount, NewStart, NewCount int
	// Lines are the chunk's lines in diff order: its deletions, then its
	// additions.
	Lines []ChangedLine
}

// diffSource is what a diff is laid out from: its hunks, and, when the whole
// modified file is known, its length and lines, so a gap after the last hunk
// can be offered and filled.
type diffSource struct {
	hunks    []hunk
	newLines []string
}

// gapBefore measures the run between two hunks, or above the first with prev
// nil. It is nil when there is nothing to fill in, or when old and new
// disagree on the run's length and it cannot be numbered.
func gapBefore(prev *hunk, h hunk) *Gap {
	from, oldFrom := 1, 1
	if prev != nil {
		from, oldFrom = prev.newStart+prev.newCount, prev.oldStart+prev.oldCount
	}
	to := h.newStart - 1
	if to < from || h.oldStart-oldFrom != h.newStart-from {
		return nil
	}
	return &Gap{key: from, From: from, To: to, OldFrom: oldFrom}
}

// gapAfter is the run after the last hunk, known only with the whole file.
// With no hunks at all, the whole file is that run.
func (s diffSource) gapAfter() *Gap {
	if s.newLines == nil {
		return nil
	}
	from, oldFrom := 1, 1
	if len(s.hunks) > 0 {
		last := s.hunks[len(s.hunks)-1]
		from, oldFrom = last.newStart+last.newCount, last.oldStart+last.oldCount
	}
	if from > len(s.newLines) {
		return nil
	}
	return &Gap{key: from, From: from, To: len(s.newLines), OldFrom: oldFrom}
}

// rows lays the diff out as one column, with what has been filled into each
// gap (by its key) laid in where the diff left a hole. A gap still open keeps
// a separator row; one filled in whole has none, the text either side now
// being continuous. Every run of changes is numbered as a chunk.
func (s diffSource) rows(filled map[int][]string) []diffRow {
	var out []diffRow
	for i, h := range s.hunks {
		var prev *hunk
		if i > 0 {
			prev = &s.hunks[i-1]
		}
		g := gapBefore(prev, h)
		out = appendGap(out, g, filled, g == nil && i > 0)
		out = append(out, h.rows...)
	}
	out = appendGap(out, s.gapAfter(), filled, false)
	numberChunks(out)
	return out
}

// appendGap lays in what has been filled into g and a separator for the rest
// of it, or, with g nil, a bare separator when one is wanted between hunks.
func appendGap(out []diffRow, g *Gap, filled map[int][]string, bare bool) []diffRow {
	if g == nil {
		if bare {
			out = append(out, diffRow{kind: rowSeparator})
		}
		return out
	}
	got := filled[g.key]
	for n, text := range got {
		out = append(out, diffRow{kind: rowContext, text: text, oldLine: g.OldFrom + n, newLine: g.From + n})
	}
	if g.From+len(got) <= g.To {
		rest := *g
		rest.From, rest.OldFrom = g.From+len(got), g.OldFrom+len(got)
		out = append(out, diffRow{kind: rowSeparator, gap: &rest})
	}
	return out
}

func numberChunks(rows []diffRow) {
	chunk, in := 0, false
	for i := range rows {
		changed := rows[i].kind == rowAdded || rows[i].kind == rowDeleted
		if changed && !in {
			chunk++
		}
		in = changed
		if changed {
			rows[i].chunk = chunk
		}
	}
}

// Collapse leaves unchanged runs out of a diff of two documents, the way
// CodeMirror's collapseUnchanged does: Margin lines stay next to each change,
// and a run is left out only when that hides at least MinSize lines (and at
// least one).
type Collapse struct{ Margin, MinSize int }

// docsSource diffs two documents into hunks: everything when collapse is
// nil, or, with it, every unchanged run left out but margin lines next to a
// change, when that leaves out at least minSize lines.
func docsSource(original, modified string, collapse *Collapse) diffSource {
	oldLines, newLines := splitDoc(original), splitDoc(modified)
	var all []diffRow
	oldLine, newLine := 1, 1
	for _, e := range diffLines(oldLines, newLines, maxDiffEdits) {
		r := hunkRow(string(" -+"[e.op])+e.text, &oldLine, &newLine)
		all = append(all, r)
	}
	src := diffSource{newLines: newLines}
	keep := keptRows(all, collapse)
	oldLine, newLine = 1, 1
	for i := 0; i < len(all); {
		if !keep[i] {
			oldLine, newLine = all[i].oldLine+1, all[i].newLine+1
			i++
			continue
		}
		h := hunk{oldStart: oldLine, newStart: newLine}
		for ; i < len(all) && keep[i]; i++ {
			r := all[i]
			if r.oldLine != 0 {
				h.oldCount++
				oldLine = r.oldLine + 1
			}
			if r.newLine != 0 {
				h.newCount++
				newLine = r.newLine + 1
			}
			h.rows = append(h.rows, r)
		}
		src.hunks = append(src.hunks, h)
	}
	return src
}

// splitDoc is a document's lines, with no empty last line for a final line
// end, as View shows it.
func splitDoc(text string) []string {
	if text = normalize(text); text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// keptRows marks the rows a collapse leaves in.
func keptRows(rows []diffRow, collapse *Collapse) []bool {
	keep := make([]bool, len(rows))
	for i := range keep {
		keep[i] = collapse == nil || rows[i].kind != rowContext
	}
	if collapse == nil {
		return keep
	}
	for start := 0; start < len(rows); {
		if rows[start].kind != rowContext {
			start++
			continue
		}
		end := start
		for end < len(rows) && rows[end].kind == rowContext {
			end++
		}
		before, after := collapse.Margin, collapse.Margin
		if start == 0 {
			before = 0
		}
		if end == len(rows) {
			after = 0
		}
		if end-start-before-after < max(collapse.MinSize, 1) {
			before = end - start
		}
		for i := start; i < end; i++ {
			keep[i] = i < start+before || i >= end-after
		}
		start = end
	}
	return keep
}

// chunksOf lists the chunks of laid-out rows.
func chunksOf(rows []diffRow) []Chunk {
	var out []Chunk
	oldNext, newNext := 1, 1
	for _, r := range rows {
		if r.chunk == 0 {
			if r.oldLine != 0 {
				oldNext = r.oldLine + 1
			}
			if r.newLine != 0 {
				newNext = r.newLine + 1
			}
			continue
		}
		if len(out) < r.chunk {
			out = append(out, Chunk{OldStart: oldNext, NewStart: newNext})
		}
		c := &out[r.chunk-1]
		if r.kind == rowDeleted {
			if c.OldCount == 0 {
				c.OldStart = r.oldLine
			}
			c.OldCount++
			oldNext = r.oldLine + 1
		} else {
			if c.NewCount == 0 {
				c.NewStart = r.newLine
			}
			c.NewCount++
			newNext = r.newLine + 1
		}
		c.Lines = append(c.Lines, changedLine(r))
	}
	return out
}

// changedLine names a changed row the way Chunk.Lines does.
func changedLine(r diffRow) ChangedLine {
	if r.kind == rowDeleted {
		return ChangedLine{OldSide, r.oldLine, r.text}
	}
	return ChangedLine{NewSide, r.newLine, r.text}
}

// side is one document of a laid-out diff, with the rows that carry an
// affordance.
type side struct {
	text   string
	labels []string
	rows   []diffRow
	// gaps and reverts place affordances: a separator's gap by its row, and
	// a chunk's first row by the chunk.
	gaps    map[int]*Gap
	reverts map[int]int
}

// unified is the single-column layout, with a two-column gutter: the old
// number then the new one, each blank on a side the row is not in.
func unified(rows []diffRow) side {
	width := 0
	for _, r := range rows {
		width = max(width, len(strconv.Itoa(max(r.oldLine, r.newLine))))
	}
	return layoutSide(rows, true, func(r diffRow) string {
		if r.oldLine == 0 && r.newLine == 0 {
			return ""
		}
		return fmt.Sprintf("%*s %*s", width, lineNumber(r.oldLine), width, lineNumber(r.newLine))
	})
}

// split is the side-by-side layout: row N of one side is row N of the other.
// An unchanged row sits on both; a chunk pairs its deletions with its
// additions by position, and the shorter column is padded with fillers. The
// affordances live on the right, the new side.
func split(rows []diffRow) (left, right side) {
	var l, r []diffRow
	for i := 0; i < len(rows); {
		chunk := rows[i].chunk
		if chunk == 0 {
			l, r = append(l, rows[i]), append(r, rows[i])
			i++
			continue
		}
		var dels, adds []diffRow
		for ; i < len(rows) && rows[i].chunk == chunk; i++ {
			if rows[i].kind == rowDeleted {
				dels = append(dels, rows[i])
			} else {
				adds = append(adds, rows[i])
			}
		}
		for pair := range max(len(dels), len(adds)) {
			l = append(l, pairAt(dels, pair, chunk))
			r = append(r, pairAt(adds, pair, chunk))
		}
	}
	left = layoutSide(l, false, func(r diffRow) string { return lineNumber(r.oldLine) })
	right = layoutSide(r, true, func(r diffRow) string { return lineNumber(r.newLine) })
	return left, right
}

func pairAt(run []diffRow, i, chunk int) diffRow {
	if i < len(run) {
		return run[i]
	}
	return diffRow{kind: rowFiller, chunk: chunk}
}

func lineNumber(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// layoutSide builds one document; withAffordances places the expand and
// revert affordances on it.
func layoutSide(rows []diffRow, withAffordances bool, label func(diffRow) string) side {
	s := side{rows: rows, gaps: map[int]*Gap{}, reverts: map[int]int{}}
	texts := make([]string, len(rows))
	for i, r := range rows {
		texts[i] = r.text
		s.labels = append(s.labels, label(r))
		if !withAffordances {
			continue
		}
		if r.gap != nil {
			s.gaps[i] = r.gap
		}
		if _, seen := s.reverts[r.chunk]; r.chunk != 0 && !seen {
			s.reverts[r.chunk] = i
		}
	}
	s.text = strings.Join(texts, "\n")
	return s
}
