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
  `Theme` changes (spans, search matches and the selection are painted beneath the images and never join the
  key). The two
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
- **A diff is highlighted a run of rows at a time, and runs only at its gaps**: the rows between two gaps are
  parsed as one text, apart from the rows past either gap, so a comment or string the lines left out would have
  closed ends at the gap. That parse starts from scratch each time; only a diff with no gaps left is reparsed
  incrementally. Within a run, a unified diff still parses its deleted and added rows as one interleaved text,
  so a change that opens or closes a comment can color the rows of the other side, and a run that starts inside
  a comment opened in a gap above it shows that comment as code.
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
