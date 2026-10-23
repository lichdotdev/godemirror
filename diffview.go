package godemirror

import (
	"slices"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/glyphs"
)

// DiffOptions are what a DiffView asks of its caller. Every field may be left
// zero.
type DiffOptions struct {
	// Collapse leaves unchanged runs out of a diff set with SetDocs; nil shows
	// every line. A patch set with SetPatch leaves out what git left out.
	Collapse *Collapse

	// Expand asks for the lines of a gap of a patch, which the DiffView does
	// not have; hand them back with FillGap. nil offers no way to expand a
	// patch's gaps. A diff set with SetDocs fills its gaps itself.
	Expand func(Gap)
	// Revert asks to put a chunk back the way the original had it. The
	// DiffView changes nothing itself: set the diff again once the files
	// have changed. nil offers no revert.
	Revert func(Chunk)
	// ExpandLabel is the text of the default expand affordance for a gap of
	// hidden lines; nil is "N unchanged lines".
	ExpandLabel func(hidden int) string
	// ExpandWidget and RevertWidget replace the default affordances: each
	// gets what it acts on and the function its click calls.
	ExpandWidget func(g Gap, expand func()) layout.Widget
	RevertWidget func(c Chunk, revert func()) layout.Widget

	// HighlightChanges tints the words that changed within a line, in each
	// deleted line and the added line it pairs with by position in its
	// chunk, with the theme's ChangeColors.Changed. Lines with no word in
	// common are left whole.
	HighlightChanges bool

	// StyleTags add rules to the Lezer grammars they name, on both sides,
	// as Content.StyleTags does for a View.
	StyleTags []StyleTags
}

// DiffView shows the difference between two versions of a file, unified in
// one column or side by side, the way CodeMirror's merge view does: changed
// lines tinted in the theme's Diff colors, each run of them a chunk that
// lights up under the pointer. Build one with NewDiffView or
// NewDiffViewFromFonts, give it a diff with SetDocs or SetPatch, and call
// Layout every frame. Like a View, it belongs to the goroutine that lays it
// out.
type DiffView struct {
	// Split shows the diff side by side, the original on the left; otherwise
	// it is one column with the deletions above the additions.
	Split bool
	// Wrap breaks long lines instead of scrolling them sideways.
	Wrap bool
	// Nested is View.Nested: the diff sits in a parent that scrolls
	// vertically.
	Nested bool

	opts    DiffOptions
	unified *View
	split   *Split

	// shown is whether a diff was set; key names its file and body its
	// version, so a new version of the same file updates in place.
	shown     bool
	key, body string
	filename  string
	src       diffSource
	// filled is what has been filled into each gap, by its key, and gen
	// counts the diffs set, so an answer for an older one is dropped.
	filled map[int][]string
	gen    int

	rows   []diffRow
	chunks []Chunk
	blocks []DiffBlock
	// onScreen are the rows each view shows, which its selection indexes.
	onScreen            map[*View]*viewRows
	affordances         affordances
	unifiedSide         side
	leftSide, rightSide side
}

// NewDiffView builds a diff view drawing family at size, resolved as New
// resolves it. It fails as New does.
func NewDiffView(family string, size unit.Sp, opts DiffOptions) (*DiffView, error) {
	r, err := glyphs.New(family)
	if err != nil {
		return nil, err
	}
	return newDiffView(r, size, opts), nil
}

// NewDiffViewFromFonts builds a diff view drawing fonts at size, without
// looking for installed fonts. It fails as NewFromFonts does.
func NewDiffViewFromFonts(fonts Fonts, size unit.Sp, opts DiffOptions) (*DiffView, error) {
	r, err := rasterizerOf(fonts)
	if err != nil {
		return nil, err
	}
	return newDiffView(r, size, opts), nil
}

func newDiffView(r *glyphs.Rasterizer, size unit.Sp, opts DiffOptions) *DiffView {
	d := &DiffView{
		opts:    opts,
		unified: newView(r, size),
		split:   &Split{Left: newView(r, size), Right: newView(r, size)},
	}
	d.affordances.labels = map[labelKey]*rowImage{}
	d.onScreen = map[*View]*viewRows{d.unified: {}, d.split.Left: {}, d.split.Right: {}}
	return d
}

// SetTheme colors the diff with t, its code and its changes (t.Diff), from
// the next Layout. A DiffView starts in DarkTheme.
func (d *DiffView) SetTheme(t Theme) {
	d.unified.Theme = t
	d.split.SetTheme(t)
}

// SetDocs shows the diff between two versions of a document, worked out here
// line by line. filename picks the language both are highlighted as, and
// names the file: a diff of the same filename replaces the one shown in
// place, keeping the scroll, and another opens from its top. Either way, what
// was expanded is collapsed again. Two documents more than 10,000 lines of
// edits apart in one stretch show that stretch as a single chunk.
func (d *DiffView) SetDocs(filename, original, modified string) {
	d.show(filename, filename, original+"\x00"+modified, docsSource(original, modified, d.opts.Collapse))
}

// SetPatch shows one file of a patch. Like SetDocs, a new version of the file
// shown (the same Path, another diff) replaces it in place, and another file
// opens from its top.
func (d *DiffView) SetPatch(p FilePatch) {
	d.show(p.Path, p.Path, p.body, p.source())
}

func (d *DiffView) show(key, filename, body string, src diffSource) {
	if d.shown && key == d.key && body == d.body {
		return
	}
	same := d.shown && key == d.key
	d.shown, d.key, d.body, d.filename, d.src = true, key, body, filename, src
	d.filled = map[int][]string{}
	d.gen++
	if !same {
		d.affordances.reset()
	}
	d.layOut()
	if same {
		d.update()
		return
	}
	u, l, r := d.contents()
	d.unified.SetContent(u)
	d.split.Left.SetContent(l)
	d.split.Right.SetContent(r)
	for _, rows := range d.onScreen {
		rows.shown = rows.next
	}
}

// FillGap lays lines into g, the gap DiffOptions.Expand asked for, in place.
// They may come in parts: a later part is FillGap(g2, more), with g2 a copy
// of g whose From has moved past the lines filled so far. A part that does
// not start where the gap now starts (two expands of one gap in flight), or
// that answers a diff since replaced, is dropped; lines past the gap's end
// are ignored.
func (d *DiffView) FillGap(g Gap, lines []string) {
	got := d.filled[g.key]
	if g.gen != d.gen || len(lines) == 0 || g.From != g.key+len(got) {
		return
	}
	d.filled[g.key] = append(got, lines[:min(len(lines), g.To-g.From+1)]...)
	d.layOut()
	d.update()
}

// SetSearch is View.SetSearch for the diff: the same query looks through
// every view it has, unified and both sides of the split, and follows the
// diff as it is replaced, expanded or given blocks. A Match's Pos counts the
// rows of the view on screen, deletions and additions included, not file
// lines; select it with FindNext and read the file lines off SelectedRange.
func (d *DiffView) SetSearch(q SearchQuery) error {
	m, err := compileSearch(q)
	d.unified.useSearch(m)
	d.split.Left.useSearch(m)
	d.split.Right.useSearch(m)
	return err
}

// FindNext is View.FindNext in the view on screen: the unified one, or, side
// by side, the side that holds the selection, or else the right one. A match
// lies in one side, so a search that wraps never leaves it.
func (d *DiffView) FindNext() (Match, bool) { return d.searched().FindNext() }

// Matches are View.Matches of the view FindNext searches: what a search box
// counts ("3 of 12"). Their lines are that view's rows, not file lines.
func (d *DiffView) Matches() []Match { return d.searched().Matches() }

// FindPrevious is View.FindPrevious in the view FindNext searches.
func (d *DiffView) FindPrevious() (Match, bool) { return d.searched().FindPrevious() }

func (d *DiffView) searched() *View {
	switch {
	case !d.Split:
		return d.unified
	case !d.split.Left.sel.empty():
		return d.split.Left
	}
	return d.split.Right
}

// Chunks are the runs of changed lines of the diff set last, in order, with
// lines filled into a gap counted. Like everything set on a DiffView, they
// reach the screen once highlighted (see UpdateContent).
func (d *DiffView) Chunks() []Chunk { return slices.Clone(d.chunks) }

// Layout draws the diff into the whole of gtx.Constraints.Max, unified or
// side by side, and then runs what its affordances were clicked for:
// DiffOptions.Expand and Revert are called from here, after the frame is
// drawn. Its error means what View.Layout's does.
func (d *DiffView) Layout(gtx layout.Context) (layout.Dimensions, error) {
	defer d.affordances.runQueued()
	if d.Split {
		d.split.Wrap, d.split.Nested = d.Wrap, d.Nested
		return d.split.Layout(gtx)
	}
	d.unified.Wrap, d.unified.Nested = d.Wrap, d.Nested
	return d.unified.Layout(gtx)
}

// layOut lays the rows out, unified and side by side.
func (d *DiffView) layOut() {
	d.rows = d.src.rows(d.filled)
	if d.opts.HighlightChanges {
		markChanges(d.rows)
	}
	for _, r := range d.rows {
		if r.gap != nil {
			r.gap.gen = d.gen
		}
	}
	d.chunks = chunksOf(d.rows)
	d.unifiedSide = unified(d.rows)
	d.leftSide, d.rightSide = split(d.rows)
}

// update swaps in the rows laid out last, keeping the scroll.
func (d *DiffView) update() {
	u, l, r := d.contents()
	d.unified.UpdateContent(u)
	d.split.Left.UpdateContent(l)
	d.split.Right.UpdateContent(r)
}

// contents are the three views' contents for the rows laid out last, which
// become the rows each view will show next.
func (d *DiffView) contents() (u, l, r Content) {
	both := func(Side) bool { return true }
	u = d.content(d.unifiedSide, d.unified, both)
	l = d.content(d.leftSide, d.split.Left, func(s Side) bool { return s == OldSide })
	r = d.content(d.rightSide, d.split.Right, func(s Side) bool { return s == NewSide })
	return u, l, r
}

// content is s for v, with the blocks anchored on the sides it shows.
func (d *DiffView) content(s side, v *View, shows func(Side) bool) Content {
	d.onScreen[v].next = s.rows
	c := Content{Filename: d.filename, Text: s.text, Labels: s.labels, Blocks: d.blocksOf(s, shows), StyleTags: d.opts.StyleTags}
	if n := len(s.rows); n > 0 && s.rows[n-1].text == "" {
		// A final line end starts no line of its own in a View, so an empty
		// last row (a separator, a filler) needs one more.
		c.Text += "\n"
	}
	for i, r := range s.rows {
		c.Decorations = append(c.Decorations, rowDecoration(r))
		if r.kind == rowSeparator {
			c.breaks = append(c.breaks, i)
		}
	}
	c.Widgets = d.widgets(s, v)
	for _, w := range c.Widgets {
		// An expander rides its separator, whose rule would run through it.
		if w.Placement == AfterText {
			c.Decorations[w.Line].role = roleNone
		}
	}
	return c
}

// rowDecoration is how a row is drawn; its colors come from the theme.
func rowDecoration(r diffRow) LineDecoration {
	d := LineDecoration{HoverGroup: r.chunk, Spans: r.spans}
	switch r.kind {
	case rowAdded:
		d.role = roleAdded
	case rowDeleted:
		d.role = roleDeleted
	case rowFiller:
		d.role = roleFiller
	case rowSeparator:
		d.role = roleSeparator
	}
	return d
}
