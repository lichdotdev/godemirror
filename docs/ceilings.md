# Known Ceilings

Deliberate limits and shortcuts, and the traps they set. A bullet earns its place by naming something that breaks
work when nobody knows it and that the call site never shows. The mechanism stays in the code and its godoc; this
file is the trap alone.

- **Fonts resolve through fontconfig** (`glyphs`): `New` and `NewSplit` run `fc-match` for the family's four
  styles, and a character the family lacks is looked up with `fc-list :charset=X` and ranked by
  `fc-match -s monospace`. Gio itself cannot resolve a family like a Nerd Font by name. A machine without
  fontconfig (stock macOS, Windows) gets `ErrNoFontconfig` from `New`; `NewFromFonts` takes the font files
  instead and has no fallback at all, so a character those files lack is drawn as a box.
- **A missing glyph is a box, and the miss sticks**: a character no font covers, or whose fallback lookup fails
  (fontconfig gone mid-session, a broken font file), is drawn as a box and never fails `Layout`. The miss is
  cached per character until the size changes, so a font installed while the view is open shows only after
  that. The fonts' own `.notdef` is not used: Go Mono, JetBrains Mono and Noto Sans Mono leave it empty.
- **One rune is one cell**: CJK and emoji are drawn in a single cell and overlap their neighbour, and a
  selection or a click there lands by cell, not by character. Fixing it needs a width table in `document.add`
  and everywhere a column is turned into pixels.
- **Highlighting is chroma, not lezer**: chroma's lexers tag every call as a function name, so calls take the
  Definition color where CodeMirror colors only the declaration, and some lexers (Go) leave type names used as
  values uncolored. A theme ported from CodeMirror will look busier on calls.
- **A new document shows plain before it shows colored**: `SetContent` draws the text at once and swaps the
  colors in when the goroutine finishes, about half a second for a 13k-line file. `UpdateContent` does the
  opposite: the *old* content stays on screen until the new one is colored. For those milliseconds anything
  that reads the view (a click, `SelectedLines`, `Selection`, `LineText`) reads the old content, and line
  indices from the new `Content` do not apply yet: `Select` and `ScrollToLine` called then land on the old
  content's lines, clamped to its length.
- **Every block widget is laid out every frame, visible or not**, so the document's height is known before
  anything is placed. Cost grows with the number of blocks, not with what is on screen; and a block scrolled out
  of view has its ops dropped, so an Editor inside it loses keyboard focus while it is off screen.
- **The view claims vertical scroll at its own ends**: Shift+wheel arrives as a vertical delta and Gio clamps a
  delta to the handler's range before the handler sees it, so the vertical range is widened to admit the
  horizontal one. Inside a parent that also scrolls (a list of diff cards), a wheel at the view's top or bottom
  edge does not pass to the parent while the view still has room to scroll sideways.
- **Row images are cached by what they hold**: `rowKey` is the line, the wrapped segment, the first column and
  the span of cells, and a view drops them all when its own font size in pixels or its `Theme` changes (spans,
  search matches and the selection are painted beneath the images and never join the key). The two
  sides of a `Split` share one rasterizer, so a view must compare against the size it composed at, never ask the
  rasterizer whether the size changed: the first side to set it would be the only one told. Anything new that
  changes a row's pixels (a per-cell style, a ligature) must join the key or drop the cache, or the old image
  keeps being drawn. The three bugs this has caused (wrap, resize, theme) showed only after a toggle or a
  resize, never on first paint.
- **A diff past 10,000 edits is one chunk** (`maxDiffEdits`): `SetDocs` runs Myers' diff, which costs
  O((N+M)·D). A stretch of the two documents that needs more edits than that shows as all of it deleted and all
  of it inserted, not as its smallest diff.
- **Everything a `DiffView` is told goes through `UpdateContent`**: a new version of the file, `FillGap`,
  `SetBlocks`. The rows on screen stay until the new ones are highlighted, so an expand or a new thread shows
  a moment late on a big file. `SelectedRange` and `SelectedChanges` read the rows on screen; `Chunks` reads the
  diff set last.
- **A `DiffView` calls back after the frame**: `DiffOptions.Expand` and `Revert` run at the end of `Layout`,
  never while it paints, so a callback may set a new diff at once. A `DiffBlock` on a line inside a gap is not
  shown until the gap is filled.
- **A held drag scrolls the view; `Select` does not**: dragging a selection past an edge scrolls the view for as
  long as the button is held there, and each such frame asks for the next with `op.InvalidateCmd`, so the host
  must honor it (a window that redraws only on input events scrolls only as the pointer moves). The time of a
  step is `gtx.Now`, so a caller that lays out with a fixed `Now` sees no scrolling. `Select` leaves the scroll
  alone (pair it with `ScrollToLine`). In a `Split` a selection lives on one side; starting one with the pointer
  clears the other, but `Select` on one side does not.
- **Widgets have fixed boxes**: a `LineWidget` gets one row's height, and a `BlockWidget` the visible code
  area's width, so a block does not scroll sideways with the code under it.
- **The first text Gio shapes costs about 28ms**, once per process (font loading and shaper caches). A view
  whose first widget with a label appears on hover pays it on that hover; an app warms it with its other UI.
- **Search is per line, and runs on the UI goroutine**: `SetSearch` looks through one line at a time, so a regexp
  never matches across a line end (`\s` and `\n` cannot join two lines) and a literal containing a newline finds
  nothing. Matches do not overlap (leftmost first) and an empty regexp match is dropped. The whole document is one
  pass on the caller's goroutine at every `SetSearch` and every change of content, about 25ms for 13,000 lines
  of 70 characters, so a search box that searches as you type pays that per keystroke on a big file. A match
  count in the millions costs memory, 32 bytes each. Between `UpdateContent` and its swap `Matches`, `FindNext`
  and `FindPrevious` read the old content's lines, as the rest of the view does.
- **The current match is the selection**: a selection that is exactly a match is lit in `Theme.SearchCurrent`
  and `Theme.Selection` is not painted over it; drag it wider or narrower and it is a plain selection again. In
  a `DiffView` a `Match`'s lines count the rows of the view on screen (deletions and additions included), not
  file lines, and `FindNext` stays on the side of a split that holds the selection.
