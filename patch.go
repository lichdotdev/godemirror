package godemirror

import (
	"fmt"
	"strconv"
	"strings"
)

// FilePatch is one file of a unified diff in git's format, as ParsePatch reads
// it. DiffView.SetPatch shows it.
type FilePatch struct {
	// Path is the file's path on the new side, or on the old side for a
	// deleted file; it also picks the language the diff is highlighted as.
	Path string
	// OldPath is the file's path on the old side, "" for an added file.
	OldPath        string
	Added, Deleted int

	hunks []hunk
	// body is this file's part of the patch, which tells two versions of the
	// same file's diff apart.
	body string
}

type rowKind uint8

const (
	rowContext rowKind = iota
	rowAdded
	rowDeleted
	rowSeparator
	rowFiller
)

// diffRow is one row of a laid-out diff.
type diffRow struct {
	kind rowKind
	text string
	// oldLine and newLine are the row's numbers in each file, 1-based, 0 on
	// a side it is not in.
	oldLine, newLine int
	// chunk is the run of changes the row belongs to, 1-based, 0 for none.
	chunk int
	// gap is the unchanged run a separator stands for, when it can be
	// expanded.
	gap *Gap
	// spans are the words that changed against the row it pairs with.
	spans []Span
}

// hunk is a stretch of the diff that is shown. Its starts are the line
// numbers its first row has on each side; a side with no rows starts at the
// line the hunk sits before.
type hunk struct {
	oldStart, oldCount, newStart, newCount int
	rows                                   []diffRow
}

// ParsePatch reads every file of a unified diff in git's format (the output of
// git diff), in order. Text before the first "diff --git" line is skipped. It
// fails on a hunk header it cannot read.
func ParsePatch(patch string) ([]FilePatch, error) {
	var r patchReader
	for line := range strings.Lines(patch) {
		if err := r.read(line); err != nil {
			return nil, err
		}
	}
	r.flush()
	return r.files, nil
}

// patchReader reads a patch a line at a time: the file it is in, the hunk,
// and the next line number on each side.
type patchReader struct {
	files            []FilePatch
	cur              *FilePatch
	h                *hunk
	oldLine, newLine int
	body             strings.Builder
}

func (r *patchReader) flush() {
	if r.cur != nil {
		r.cur.body = r.body.String()
		r.files = append(r.files, *r.cur)
	}
	r.body.Reset()
}

func (r *patchReader) read(line string) error {
	if strings.HasPrefix(line, "diff --git ") {
		r.flush()
		r.cur, r.h = &FilePatch{Path: newPathOf(line)}, nil
	}
	if r.cur == nil {
		return nil
	}
	r.body.WriteString(line)
	line = strings.TrimSuffix(line, "\n")
	switch {
	case r.h == nil && strings.HasPrefix(line, "--- "):
		if line != "--- /dev/null" {
			r.cur.OldPath = strings.TrimPrefix(line, "--- a/")
		}
	case r.h == nil && strings.HasPrefix(line, "+++ ") && line != "+++ /dev/null":
		r.cur.Path = strings.TrimPrefix(line, "+++ b/")
	case strings.HasPrefix(line, "@@"):
		parsed, err := parseHunkHeader(line)
		if err != nil {
			return fmt.Errorf("%s: %w", r.cur.Path, err)
		}
		r.cur.hunks = append(r.cur.hunks, parsed)
		r.h = &r.cur.hunks[len(r.cur.hunks)-1]
		r.oldLine, r.newLine = parsed.oldStart, parsed.newStart
	case r.h == nil, strings.HasPrefix(line, `\`):
	default:
		row := hunkRow(line, &r.oldLine, &r.newLine)
		r.h.rows = append(r.h.rows, row)
		switch row.kind {
		case rowAdded:
			r.cur.Added++
		case rowDeleted:
			r.cur.Deleted++
		}
	}
	return nil
}

// newPathOf reads the new path off "diff --git a/<old> b/<new>".
func newPathOf(header string) string {
	header = strings.TrimSuffix(header, "\n")
	if i := strings.LastIndex(header, " b/"); i >= 0 {
		return header[i+3:]
	}
	return strings.TrimPrefix(header, "diff --git ")
}

// hunkRow reads one line of a hunk, advancing the line counters past it.
func hunkRow(line string, oldLine, newLine *int) diffRow {
	switch {
	case strings.HasPrefix(line, "+"):
		r := diffRow{kind: rowAdded, text: line[1:], newLine: *newLine}
		*newLine++
		return r
	case strings.HasPrefix(line, "-"):
		r := diffRow{kind: rowDeleted, text: line[1:], oldLine: *oldLine}
		*oldLine++
		return r
	}
	r := diffRow{kind: rowContext, text: strings.TrimPrefix(line, " "), oldLine: *oldLine, newLine: *newLine}
	*oldLine++
	*newLine++
	return r
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
	// git starts a side with no lines at the line before the hunk; a hunk
	// here starts at the line it sits before.
	if h.oldCount == 0 {
		h.oldStart++
	}
	if h.newCount == 0 {
		h.newStart++
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

// source is what a patch knows of its file: the hunks, and no file length.
func (p FilePatch) source() diffSource {
	return diffSource{hunks: p.hunks}
}
