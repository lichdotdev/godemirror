# Known Ceilings

Deliberate limits and shortcuts, and the traps they set. A bullet earns its place by naming something that breaks
work when nobody knows it and that the call site never shows. The mechanism stays in the code and its godoc; this
file is the trap alone.

- **Fonts resolve through fontconfig where it is installed** (`glyphs`): `New` and `NewSplit` run `fc-match` for
  the family's four styles, and a character the family lacks is looked up with `fc-list :charset=X` and ranked
  by `fc-match -s monospace`, a ranking read once per process. Gio itself cannot resolve a family like a Nerd
  Font by name. A view tries the fallbacks it already loaded before looking up again, so a character two
  fallbacks cover is drawn from the one loaded first, not the one fontconfig ranks closer. Each font is read
  once per process and kept until it exits; two fonts of one collection read its file twice. `NewFromFonts`
  takes the font files instead and has no fallback at all, so a character those files lack is drawn as a box.
- **Without fontconfig, fonts are found by scanning their directories** (`glyphs`, Windows and macOS always, Linux
  when `fc-match` is missing): the first `New` of a process reads every font in the platform's font directories
  (0.1 s for 1,400 fonts), and a font installed afterwards is not found until the process restarts. A family
  matches only the exact name a font carries (case aside), with no fuzzy match: where `fc-match` always picks
  something, an unknown family is `ErrFontNotFound`. `monospace` is a fixed per-platform list (Cascadia Mono,
  Consolas, Courier New on Windows; Menlo, Monaco, Courier New on macOS). A style outside regular, bold,
  italic and bold italic (Light, SemiBold) is never picked, and a variable font draws its default instance for
  every style. A missing character is looked up across the fonts in a fixed order (the monospace list, then a
  per-platform CJK and symbol list, then the rest): Han characters take the first CJK family installed
  (Microsoft YaHei, PingFang SC), whatever the text's language. A character no font covers reads every font
  once per process, about 0.1 s for 1,400 fonts.
- **A missing glyph is a box, and the miss sticks**: a character no font covers, or whose fallback lookup fails
  (fontconfig gone mid-session, a broken font file), is drawn as a box and never fails `Layout`. The miss is
  cached per character until the size changes, so a font installed while the view is open shows only after
  that. The fonts' own `.notdef` is not used: Go Mono, JetBrains Mono and Noto Sans Mono leave it empty.
- **A character is one or two cells, never zero**: the width table (`glyphs.Width`, Unicode 18.0.0's East
  Asian Wide and Fullwidth plus Emoji_Presentation) gives the wide ones two. Zero-width characters (combining
  marks, ZWJ, variation selectors) still take a cell each: a combining accent leaves a gap after its letter,
  and an emoji sequence (a flag, a family, a skin tone) is its parts side by side. A text emoji made emoji by
  U+FE0F (❤️) stays one cell. Color emoji fonts (Noto Color Emoji is CBDT) cannot be rasterized by `x/image`:
  fontconfig still picks them, so those emoji are blank, two cells wide, not boxes. A wide glyph is drawn from
  the left of its two cells, so a CJK font with a 1em advance leaves a sliver on its right.
- **Eight languages are parsed by Lezer, two by stream modes; everything else is chroma**: a file whose extension (any case)
  `@codemirror/language-data` gives to CSS, Go, HTML, Java, JavaScript, JSX, TypeScript, TSX, Python or Rust goes
  through `internal/lezer`, a port of `@lezer/lr`, `@lezer/common` and `@lezer/highlight` running the tables of
  `@lezer/css` 1.3.9, `@lezer/go` 1.0.1, `@lezer/html` 1.3.13, `@lezer/java` 1.1.5, `@lezer/javascript` 1.5.6,
  `@lezer/python` 1.1.19 or `@lezer/rust` 1.0.3, and is tagged exactly as CodeMirror tags it, errors included.
  Those grammars' gaps show the same half-colored way they do in CodeMirror: Java's switch expressions and
  record patterns; Go's `for range 3`, and a multi-line `/* */` that does not end a statement; TypeScript's
  `declare module "x"`, `abstract new`, labeled tuples and `let v!:`. Python is matched by extension only, so a
  file named `BUILD` or `BUCK` with none is chroma's. Kotlin and shell are the stream modes of the next bullet. Every other language is tagged by chroma with the CodeMirror tag a lezer grammar gives the same construct (`tagmap.go`), but
  chroma knows less than a parse tree. Every call is tagged `Function(VariableName)` like the declaration, so a
  theme that colors only declarations colors calls too; Go type names used as values, and a declared name, stay
  `VariableName`; operators are all `Operator`. A ported theme is faithful tag for tag and still looks busier or
  flatter than in CodeMirror wherever chroma is coarser.
- **`StyleTags` reach one grammar, by the paths styleTags has**: a rule names a node by its parents, never by its
  place among its siblings, so what the tree tells apart only by position stays out of reach: Java's `a.b` is an
  `Identifier` under `FieldAccess` twice, and `RicherJavaTags` leaves both `variableName`; Python's default value
  `x=DEFAULT` is a `VariableName` under `ParamList` beside the parameter, so `RicherPythonTags` tags `DEFAULT` a
  parameter too. No scope is tracked, so only a parameter's declaration is tagged, never its uses.
- **`StyleRule.Names` reads a node's text, which CodeMirror's styleTags cannot**: the builtin sets
  (`GoBuiltinTags` and the others) tag a name by its text alone, so a local variable that shadows a builtin
  (`len := 3`) is tagged a builtin, and host globals (`console`, `window`) are left out because they differ by
  host. A rule with Names has nothing in Lezer to be checked against, so those sets are checked by tests alone,
  not by goldens; the paths they use still match as Lezer's do. A grammar's precedence quirks stay too:
  `@lezer/go` parses `a + len(s)` as `(a + len)(s)`, so that `len` is not a call and keeps the plain tag. Rules for `"javascript"`
  or `"css"` do not reach the scripts and styles inside an HTML file, which `@codemirror/lang-html` parses with
  its own copies of those grammars, as configuring them does not in CodeMirror either. Each set of rules keeps
  its own parser, so content whose `StyleTags` change is parsed from scratch once, not incrementally.
- **Kotlin and shell are CodeMirror 5 modes, and tag as coarsely as they do**: `.kt`, `.kts`, `.sh`, `.ksh`,
  `.bash` (any case) and a file named exactly `PKGBUILD` go through `internal/stream`, a port of
  `@codemirror/language` 6.13.1's `StreamLanguage` and legacy token table and of `@codemirror/legacy-modes` 6.5.5's
  `kotlin` and `shell`, tagged exactly as CodeMirror tags them. Those modes know little: Kotlin's only tags are
  keywords, a fixed list of `java.lang` and Kotlin types, `true false null this`, strings, numbers, comments,
  annotations (`meta`) and operators, every other name is `variableName` (calls included), but the one after
  `class`, `fun`, `val` and the like is `definition(variableName)`, and so is a name right after that one
  (`fun foo bar`); `!in`, `!is` and `as?` never come out as keywords. Shell tags every `$x`, `${...}` and
  `x=` as `definition(variableName)`, knows 67 commands (`variableName.standard`) and nothing of
  the others, and colors the rest of a heredoc's opening line as the heredoc; only a line that is exactly the
  delimiter closes it, so an indented one under `<<-` does not. A token starting past the 10,000th UTF-16 unit
  of a line ends that line's highlighting, as CodeMirror's `MaxLineLength` does. Only what decides a style is
  ported: no indentation (clike's contexts), no `blankLine`, `copyState` or a mode's own `tokenTable`, which
  neither mode has, so a third mode may need them. Unlike the Lezer port, every change tokenizes the whole
  file again.
- **A content change is reparsed incrementally, but the document is still rebuilt whole**: a view keeps the
  last Lezer tree and text it highlighted, and when new content in the same language comes (`SetContent` or
  `UpdateContent`, any file name), it takes the change to be the one stretch between the two texts' common prefix
  and suffix and reuses the rest of the tree, as CodeMirror reparses after an edit (`TreeFragment`, `@lezer/lr`'s
  node reuse, `parseMixed`'s reuse of nested trees). A one-character edit to 13,000 lines of Java reparses in
  about 1.2 ms instead of 50 ms, but the cells and tags of every line are still built again, so the update takes
  about 28 ms instead of 78 (chroma's pass takes about 490 ms). Lezer reuses nothing in a text under 4,096 UTF-16
  units. The incremental tree is Lezer's, not always a fresh parse's: in broken code (an unclosed string, a stray
  token) error recovery can go another way, and a node reused under a nested parse keeps the tree nested in it,
  so the colors of a broken stretch can depend on the updates that led to it, exactly as in CodeMirror; valid
  code highlights as a fresh parse does. A reparse shares nodes with the tree before it and a nested parse writes
  its tree onto them, so a tree goes to exactly one reparse and to nothing else: a view highlights one content
  at a time, in order, and a content replaced before its turn is skipped. Nested parsing is `parseMixed` without
  `stopAt`, predicate overlays or `bracketed`, and with one nest per parser
  (`Parser.WithNest`), which is all `@codemirror/lang-html`'s default nesting uses; its custom
  `nestedLanguages` and `Element` tags are not ported. HTML keeps a quirk of `@lezer/html`'s `content.js`: it
  tests `isError` on a node that has no such property, so an attribute's overlay always drops its last
  character, unclosed quote or not. A grammar's JavaScript (external tokenizers, specializers, its context tracker) is ported by
  hand and passed in `lezer.Externals`; `lezer.New` fails naming what is missing.
- **A styleTags path sees the parents a tree cursor sees, quirk included**: inside a `TreeBuffer`,
  `@lezer/common`'s `TreeCursor.matchContext` takes the last tree node the cursor stood on, often the previous
  sibling, for the buffer's parent, so a path reaching above the buffer can fail where the tree has the right
  parents. CodeMirror highlights with that cursor, so the port keeps it, and keeps the tree's exact layout
  (buffers, balanced repetitions) because the quirk depends on it. Flattening the tree, or "fixing" the cursor,
  changes what is highlighted.
- **A token's style is its tags' specs merged, color and font only**: CodeMirror applies `HighlightStyle.define`'s
  CSS; godemirror merges, in spec order, the specs a token's tags reach and draws only their color, bold, italic,
  underline and line-through. A spec's background, border or outline is dropped, so it is not in `TagStyle`, and
  a later spec cannot take back an earlier one's bold or italic, as CSS `font-weight: normal` would. Each cell
  holds its tags as one pointer to an interned set: eight bytes a cell, about 8 MB for a 13k-line file.
- **A new document shows plain before it shows colored**: `SetContent` draws the text at once and swaps the
  colors in when the goroutine finishes, about half a second for a 13k-line file read by chroma, a tenth of that
  for one parsed by Lezer. `UpdateContent` does the
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
  the span of cells, cut to the line's end. The view's width is not in it: an unwrapped row holds a page of
  `pageCols` columns, two screens rounded up to 64, so resizing redraws a row only when its page changes, and
  wrapped, only when its own segment does. A view drops them all when its own font size in pixels or its
  `Theme` changes; an `Editor`'s edit drops the images of the lines it touched and renumbers the ones after
  them, so a key still names the line it shows (spans, search matches and the selection are painted beneath
  the images and never join the key). The two sides of a `Split` share one rasterizer, so a view must compare against the size it composed at, never ask the
  rasterizer whether the size changed: the first side to set it would be the only one told. Anything new that
  changes a row's pixels (a per-cell style, a ligature) must join the key or drop the cache, or the old image
  keeps being drawn. The three bugs this has caused (wrap, resize, theme) showed only after a toggle or a
  resize, never on first paint.
- **A wrapped row hangs by a virtual indent, which CodeMirror's `lineWrapping` does not do**: a continuation
  row starts `hangOf` cells in, the line's leading spaces (a tab counts its cells) plus 2, capped at half the
  columns so a deeply indented line keeps room, and breaks at the columns left. The indent is not text: the row's
  cells, the selection, search spans, copy and positions are the line's own, and the pointer, `coordsAt` and
  `posAtCoords` add or take the hang off at the edge. A point in the indent is the row's first cell. The editor
  keeps the column it moves vertically on in pixels, so Up from a continuation row lands `hang` cells further
  into a row without one. A rectangular selection's column past a line's end is still counted from the pointer's
  place, hang not included, as it never took a wrapped row's start into account. A `LineWidget` after the text
  of a continuation row is placed past the hang.
- **A diff past 10,000 edits is one chunk** (`maxDiffEdits`): `SetDocs` runs Myers' diff, which costs
  O((N+M)·D). A stretch of the two documents that needs more edits than that shows as all of it deleted and all
  of it inserted, not as its smallest diff.
- **Everything a `DiffView` is told goes through `UpdateContent`**: a new version of the file, `FillGap`,
  `SetBlocks`. The rows on screen stay until the new ones are highlighted, so an expand or a new thread shows
  a moment late on a big file. `SelectedRange` and `SelectedChanges` read the rows on screen; `Chunks` reads the
  diff set last.
- **A diff is highlighted a run of rows at a time, and runs only at its gaps**: the rows between two gaps are
  parsed as one text, apart from the rows past either gap, so a comment or string the lines left out would have
  closed ends at the gap. That parse starts from scratch each time; only a diff with no gaps left is reparsed
  incrementally. Within a run, a unified diff still parses its deleted and added rows as one interleaved text,
  so a change that opens or closes a comment can color the rows of the other side, and a run that starts inside
  a comment opened in a gap above it shows that comment as code.
- **A `DiffView` calls back after the frame**: `DiffOptions.Expand` and `Revert` run at the end of `Layout`,
  never while it paints, so a callback may set a new diff at once. A `DiffBlock` on a line inside a gap is not
  shown until the gap is filled.
- **`DiffView.Height` leaves out blocks and is not cached**: a `DiffBlock` or `BlockWidget` is laid out only by
  `Layout`, once a frame, so its height is the caller's to add. Without `Wrap` the call is the row count times
  the row height; with it, every row is measured again each call, so a very long diff is sized when the diff, the
  width or `Wrap` change, not every frame.
- **A `GutterCell` widget is as wide as the gutter**: it is laid out with at most the gutter's width and centered
  in the cell, so a wider one is clipped. A `RevertWidget` in the gutter has to fit a cell of a few characters;
  the default, a single "↩", does.
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
- **An editor counts bytes, the read-only views count runes**: every position of an `Editor` (`ChangeSpec`,
  `SelectionRange`, `Line`, `MapPos`) is a UTF-8 byte offset into `EditorState.Doc`, whose lines are joined by
  `"\n"` whatever `LineSeparator` is, so offsets into `SliceDoc`'s CRLF text are not positions. A `View`'s `Pos`
  (`Select`, `Selection`, `Match`) counts a column in runes, a tab or a wide character as one. A place carried
  between a `View` and an `Editor` is converted (the runes of its line up to it), never passed across as is.
  `Dispatch` refuses a position inside a character, where CodeMirror's UTF-16 offsets let one split a
  surrogate pair.
- **An editor is `@codemirror/state`'s document and selection, without its extension system**: no facets,
  fields, effects, annotations, change or transaction filters; the options CodeMirror sets through facets are
  `EditorStateConfig` fields, fixed until the next `SetState`. A cursor has no bidi level and no range is
  undirectional (macOS). `Dispatch` takes one spec, so `sequential` specs do not exist. `OnUpdate` hears of
  transactions only, where `updateListener` also hears of viewport, focus and geometry changes. With
  `LineSeparator` set, a bare `"\n"` inside the text is still a line break, where CodeMirror keeps it inside
  its line.
- **An editor's document is one string, and only its highlighting is rebuilt whole**: a change copies `Doc`
  and finds its line starts again, where CodeMirror edits a tree of lines; the view builds the cells and the
  wrapping of the lines the change touched and keeps the others', with their colors and row images. A
  keystroke in the middle of 13,000 lines of Java costs the UI goroutine about 1 ms where rebuilding the
  view's document took 23. Highlighting then reparses incrementally but builds every line's cells and tags
  again, off the UI goroutine, as the ceiling on content changes says. Until it lands, the lines the change
  touched show uncolored and the others keep their old colors, even those the change recolors (a comment
  opened above them).
- **An editor has `@codemirror/commands`' standard keymap, as it binds off macOS, and nothing more**: arrows by
  character and, with Ctrl, by group; Up and Down; Page Up and Down; Home and End (to the wrapped row first,
  Home then to the line's indentation); Ctrl+Home and Ctrl+End; Shift for each; Enter; Ctrl+A; Backspace and
  Delete, with Ctrl by group; Ctrl+C, X and V for the clipboard; of `defaultKeymap`, Ctrl+Alt+Up and Down to
  add a cursor and Escape to go back to one; and of `@codemirror/search`'s `searchKeymap`, Ctrl+D to select the
  next occurrence and Ctrl+Shift+L every occurrence. There is no search panel, so none of its other keys. Ctrl is Gio's `ModShortcut`, Cmd on macOS,
  so macOS gets these bindings and none of CodeMirror's macOS ones (Cmd+arrows, Alt+arrows, the Emacs keys).
  Nothing else of `defaultKeymap` is bound yet: no syntax motion, moving or copying lines, commenting,
  bracket matching or indenting a selection, and Tab moves focus away, as without `indentWithTab`, unless
  `OnKey` is set.
- **With `OnKey` set, the editor takes every key read after it, and a key's text comes apart**: Gio gives a key to the first
  `gtx.Event` call of the frame whose filters match it, so while `OnKey` is set the editor's catch-all filter
  takes every key read after it, whatever `key.Filter`'s godoc says of an empty `Name` yielding to named
  filters (Gio v0.10.3). An app shortcut reaches the app only when it is read before the editor's `Layout`,
  and returning false from `OnKey` does not give the key back. A key's text comes as its own `key.EditEvent`,
  which taking the `key.Event` does not stop, where a browser's `preventDefault` on keydown stops the input:
  a caller typing for itself takes both. After a taken edit the window's text input believes the text was
  typed until the frame ends, and the editor drops the selection it reports for it.
- **Enter indents as CodeMirror does with no language**: the new line takes the column of its line's
  indentation, written in `IndentUnit` (two spaces by default, so a tab-indented line goes on in spaces unless
  `IndentUnit` is `"\t"`), and between `()`, `[]` or `{}` it breaks twice. No grammar indents by its syntax,
  as `@codemirror/lang-*`'s `indentNodeProp` would: the Lezer ports carry no indentation. CodeMirror's quirks
  stay: a line break at the very start of the document is indented to column 0, and the spaces before the
  cursor are dropped when its whole line, past the document's first lines, is blank. A tab stop is 4 columns,
  as the views draw it.
- **A press in an editor selects at once**: `@codemirror/view`'s mouse selection is ported (a click, a drag, a
  double click by character category, a triple click for the line and its break, Shift to extend the main
  range, Ctrl+click to add a range, or remove the cursor under it, with `AllowMultipleSelections`), but not its
  drag and drop, so a press inside the selection does not drag it elsewhere. `RectangularSelection` and
  `CrosshairCursor` are `rectangularSelection` and `crosshairCursor` with their default, Alt: no other key or
  event filter can be set, and a window manager that moves windows on Alt+drag takes the drag first. Past a
  line's 2,000th character CodeMirror stops counting a rectangle's columns and uses offsets; godemirror always
  counts columns. After a drag the range keeps a
  goal column of 0, as CodeMirror leaves one there, so Shift+Up or Down from it aims at the line's start.
- **An editor's caret and text input count the whole document**: Gio's text input counts runes from the start
  of the document, so every key, edit, selection and snippet it exchanges scans the document up to the cursor,
  about a millisecond per megabyte. An input method that changes text away from the main cursor while there
  are several leaves the others where their UTF-16 offsets were, unmapped, as CodeMirror does; a composition
  is repeated at each cursor outside the main cursor's line that has the same text before it, CodeMirror's
  behaviour with no DOM node to find the composition in. The caret is drawn only while the editor has focus,
  and blinks every 1.2 s, starting over when a transaction sets the selection.
- **An editor finds occurrences through NFKD, and selects whole characters**: Ctrl+D and Ctrl+Shift+L match
  the selection's text as `@codemirror/search`'s `SearchCursor` does, both sides NFKD-normalized: a precomposed
  `é` and `e` with a combining accent find each other, `fi` finds the ligature `ﬁ`, and an `e` finds the `e` an
  `é` decomposes to, selecting the whole `é`. The normalization is `golang.org/x/text`'s, of its own Unicode
  version, which a character newer than it may tell apart from the browser's. CodeMirror also takes the query
  with the line separator and searches lines joined by `"\n"`, so with `LineSeparator` `"\r\n"` a selection
  over a line break has no other occurrence, kept as it is. Ctrl+Shift+L selects nothing past 1,000 matches, as
  CodeMirror's does. The read-only views' `SetSearch` does not normalize.
- **An editor's undo history is CodeMirror's `history()` with its defaults, always on**: changes 500 ms apart
  or less, touching each other, typed or deleted, are one undo event (a composition always joins the event
  before it); at least 100 events are kept; none of `minDepth`, `newGroupDelay` or `joinToEvent` can be set,
  and the history cannot be turned off, only bypassed per transaction (`SkipHistory`). A dispatch with no
  `UserEvent` joins typing next to it within the delay, as in CodeMirror: give a programmatic edit
  `IsolateHistory` to make it an undo of its own. `historyKeymap` is bound as on Linux, so Ctrl+Shift+Z
  redoes on every platform, and macOS gets none of its own bindings. Effects in the history
  (`invertedEffects`) and its JSON form are not ported. A transaction made outside of `Layout` happened at
  `time.Now()` unless its `Time` says otherwise; one made by input, at the frame's `gtx.Now`.
- **An editor's decorations are lines and widgets, mapped as CodeMirror maps them**: `LineDecorationAt` is
  `Decoration.line`, which stays on its line as text is typed at its start (a new line typed there leaves it
  on the empty line before the text) and goes with the line break before it; `LineWidgetAt` is an inline
  `Decoration.widget`, dropped by a deletion across it. A line typed in the middle of a decorated region gets
  no decoration: the caller decorates it again, in `OnUpdate`, as a CodeMirror state field would with its
  effects. A decoration's `Spans` count runes of its line and do not follow edits within it, where
  CodeMirror would use mark decorations, which are not ported; nor are block widgets on an editor.
- **Scroll and line geometry are a single view's, in whole pixels**: `ScrollTop`, `SetScrollTop`,
  `LineBlock` (`LineBlockAt` on an `Editor`), `LineAtHeight` and `ContentHeight` answer what CodeMirror's
  `scrollDOM.scrollTop`, `lineBlockAt`, `lineBlockAtHeight` and `contentHeight` do, from the last `Layout`:
  between a `Dispatch` and the next frame, an `Editor` answers for the document on screen. Content set since
  the last `Layout` (`SetContent`, `SetState`) is estimated, as CodeMirror's height map estimates lines it has
  not measured, but more crudely: a row a line, wrapping and blocks left out, until the next frame places it. A line's block
  covers its rows, not the block widgets under it, which CodeMirror makes blocks of their own. `OnScroll`
  is ours: CodeMirror has a DOM scroll event, godemirror calls back after the frame that left the view
  scrolled elsewhere, the scroll a caller set included, so views that follow each other must stop where the
  position they set is the one they were told of. Views inside a `Split` or a `DiffView` neither call it nor
  answer for their place in it. CodeMirror has no three-pane view: `@codemirror/merge` 6.12.3's `MergeView`
  aligns two editors in one scroller with block-widget spacers before each unchanged stretch and places its
  revert buttons at `lineBlockAt(chunk.fromA).top`, with no connectors between the sides; an `Editor` has no
  block widgets for such spacers, so panes beside it follow its scroll instead.
- **An Editor's popups draw in its code font** (diagnostics, hover, completion): their text is rasterized cell by
  cell as code is, never through Gio's text stack, so prose shows monospaced, a popup is at most 72 cells wide
  and 24 rows tall, and what runs past is cut, not scrolled. A popup is clipped to the editor: one taller than
  the room above and below its anchor loses its last rows. The menu and the popups take no pointer: a click
  on the menu is a click on the code under it (which closes the menu), and the wheel scrolls the code.
- **Markdown from a server shows as plain text** (`lsp`): a hover's first fenced block is its code and the rest
  its text, fences, links' targets, backticks, `**`, `__`, `#` and rules dropped; lists, tables and a lone `*`
  or `_` stay as written, and nothing is bold or italic.
- **A snippet completion inserts plain text** (`lsp`): the client tells the server it takes no snippets, and an
  item that sends one anyway loses its tabstops and placeholders (`${1:x}` becomes `x`, `$0` nothing); the
  cursor lands at the end of the text, not at the first tabstop.
- **Completions filter as a fuzzy subsequence, in the order given** (`Editor.Complete`): items keep the order
  the hook gives (`lsp` sorts by `sortText`), with no ranking of their own; an `Incomplete` result is asked again
  on each character typed. Only one cursor completes: with several, nothing opens.
- **Diagnostics are mapped, not recomputed** (`Editor.SetDiagnostics`): between an edit and the server's next
  publish they move with the text around them, and one whose whole range was deleted is gone. `lsp` reads a
  publish in the version it names, through the last 32 versions; an older one is dropped. A dot marks the line
  a diagnostic starts on only, and the popup opens on a rest of half a second, the time hover takes too.
- **Ctrl+click goes to the definition while a `Definition` is set**, over a word: it no longer adds a cursor
  there (`AllowMultipleSelections`); Ctrl+click off a word still does. A definition with several locations
  goes to the first. One in another file reaches `OnDefinition` with its column in UTF-8 bytes, read from the
  file when the server counts in UTF-16; a file that cannot be read keeps the server's column as it was.
- **Out of the LSP client for now**: rename, code actions, formatting, references, signature help, document
  symbols (an outline), pull diagnostics (`textDocument/diagnostic`), workspace edits, progress and
  `window/showMessage` are not asked for or are dropped. A server's dynamic registrations are acknowledged and
  ignored, so a capability a server only registers later is never used.
- **An edit reaches the server from the UI goroutine** (`lsp.Attach`): `didChange` is written to the server's
  standard input inside the editor's `OnUpdate`, before the frame's requests, so that a request never
  overtakes the edit it is about. A server that stops reading its input while the pipe's buffer is full
  (64 KiB on Linux) stalls the frame until it reads again or exits.
