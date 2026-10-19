# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`New`, `NewSplit` and `NewDiffView` work without fontconfig**, on Windows,
  macOS and a Linux that lacks it: they read the fonts in the platform's font
  directories (`%WINDIR%\Fonts` and the user's on Windows; `/System/Library/Fonts`,
  `/Library/Fonts` and `~/Library/Fonts` on macOS; the XDG ones on Linux), match
  the family by the name each font carries, and fall back to those fonts for a
  character the family lacks, CJK included. `"monospace"` is the platform's
  own (Cascadia Mono or Consolas on Windows, Menlo on macOS). Where fontconfig
  is installed nothing changes.
- **`ErrFontNotFound`**, what `New` fails with, without fontconfig, when no
  installed font is of the family asked for.

### Changed

- **`New` no longer fails with `ErrNoFontconfig`** where fontconfig is missing.
  A caller that tested for it to fall back to `NewFromFonts` tests for
  `ErrFontNotFound` instead.

### Deprecated

- **`ErrNoFontconfig`**: `New` never returns it.

### Fixed

- **A family whose styles live in one collection file (`.ttc`) draws each
  style from its own font.** fontconfig names a font in a collection by file
  and index, and the index was dropped: every style came from the file's first
  font, so bold or italic text could come out regular, or in another family
  the collection holds.
- **A character the family lacks is drawn from the font fontconfig ranks
  first, also when that font is in a collection.** The fallback took the
  collection's first font instead: drawn bold or in another family the file
  holds, or as a box when that font lacked the character.

## [0.7.1] - 2026-10-17

### Fixed

- **CJK text no longer stalls a view, and views no longer hold a copy of each
  font apiece.** A character the family lacks asked fontconfig for a fallback
  once per distinct character, two subprocesses each inside a frame, so a diff
  full of CJK text froze for seconds; a fallback already loaded now answers for
  the characters it covers, and the monospace ranking is read once per process.
  Every view, and a `DiffView` has one per file, read the font files on its
  own: they are now read once per process and shared, 50 `DiffView`s holding
  about 1 MB where they held 61 MB.

## [0.7.0] - 2026-10-15

### Changed

- **`DarkTheme` and `LightTheme` style parameters and builtins**: a
  parameter's declaration, `definition(local(variableName))`, is italic in the
  text color, and `standard(variableName)` takes the type color, as
  `standard(typeName)` and `standard(className)` already did. A called builtin,
  `function(standard(variableName))`, keeps the function color.

## [0.6.0] - 2026-10-15

### Added

- **`RicherGoTags`, `RicherJavaScriptTags` (JavaScript and TypeScript),
  `RicherPythonTags` and `RicherRustTags`**, opt-in rules like
  `RicherJavaTags`: a parameter's declaration is
  `definition(local(variableName))`, which a theme coloring
  `definition(variableName)` still colors; Go's `make` is
  `function(standard(variableName))` where `@lezer/go` left it untagged.
- **`StyleRule.Names`** limits a rule to nodes whose text is one of them, which
  CodeMirror's styleTags cannot say: a node with any other text falls through
  to the next rule, and a rule with Names is tried before one as deep without.
- **`GoBuiltinTags`, `JavaScriptBuiltinTags`, `PythonBuiltinTags` and
  `RustBuiltinTags`** tag builtins by name with it: Go's predeclared types and
  functions, Python's built-in functions, ECMAScript's globals and
  TypeScript's primitive types, and Rust's primitive types, as
  `standard(typeName)`, `standard(className)`, `standard(variableName)` or,
  called, `function(standard(variableName))`.

### Changed

- **`RicherJavaTags` also tags a method's, constructor's or lambda's
  parameter** `definition(local(variableName))`, as the new Richer sets do.

## [0.5.0] - 2026-10-15

### Added

- **`StyleTags` add styleTags rules to a Lezer grammar**, as a CodeMirror
  caller does with `language.configure({props: [styleTags({...})]})`: set them
  on `Content.StyleTags` or `DiffOptions.StyleTags`, naming the grammar
  (`"java"`, `"javascript"`, ...) and the paths and tags of each `StyleRule`.
  They combine with the grammar's own rules as Lezer combines them. Rules for
  a language that has no Lezer grammar, or a path that does not parse, make
  `Layout` return an error.
- **`RicherJavaTags`**, opt-in rules that tag Java finer than `@lezer/java`
  does: enum constants as `constant(variableName)`, annotations as
  `annotation`, fields as `definition(propertyName)`, and package and import
  paths as `namespace`.

## [0.4.0] - 2026-10-15

### Changed

- **CSS, Go, HTML, Java, JavaScript, TypeScript, Python and Rust are
  highlighted by a port of CodeMirror's own Lezer parser.** A file of one of
  them is parsed with its `@lezer/*` grammar and tagged by its `styleTags`, as
  CodeMirror tags it: a call and a declaration, a type and a variable, are told
  apart where chroma could not, and HTML's scripts, styles and `style`/`on*`
  attributes are highlighted as JavaScript and CSS. Code a grammar cannot
  parse is left as CodeMirror leaves it.
- **Kotlin and shell scripts are highlighted as CodeMirror highlights them**,
  by a port of the `@codemirror/legacy-modes` modes (`kotlin`, `shell`) that
  CodeMirror runs through its `StreamLanguage`: `.kt`, `.kts`, `.sh`, `.ksh`,
  `.bash` and a file named `PKGBUILD`. Every other language is still read by
  chroma.
- **New content in a Lezer language is reparsed incrementally.** `SetContent`
  and `UpdateContent` reuse the tree of the content before, as CodeMirror
  reparses after an edit, nested scripts and styles included: a one-character
  edit to 13,000 lines of Java reparses in about 1.2 ms instead of 50 ms, and
  the whole update takes about 28 ms instead of 78; in a 13,000-line HTML page,
  0.4 ms instead of 71, and 30 ms instead of 102. Highlighting now runs one
  content at a time, in order, skipping one replaced before its turn.
- **A token can carry several tags, and its style merges theirs.** Each tag
  takes the spec of its most specific styled tag, and the specs apply in
  their order in the `HighlightStyle`, a later one's color, bold, italic,
  underline or line-through winning, as CSS cascades CodeMirror's classes. A
  chroma token still has one tag, and draws as before.

## [0.3.0] - 2026-10-14

### Added

- **`tags.Parse`** reads a tag back from the name `String` prints, modifiers
  included (`"function(definition(variableName))"`), so a theme kept in JSON or
  TOML can name the tags its `TagStyle`s style.

### Fixed

- **A line-through sits across the text in every font.** With fonts whose
  x-height `x/image` reports negative (DejaVu Sans Mono), a `Strikethrough`
  token was ruled below the baseline instead of across its letters.

## [0.2.0] - 2026-10-14

### Added

- **CodeMirror's highlighting tags.** The new `tags` package is
  `@lezer/highlight`'s vocabulary: every standard tag and the `Definition`,
  `Constant`, `Function`, `Standard`, `Local` and `Special` modifiers, falling
  back from specific to general the way CodeMirror's do. chroma's tokens are
  tagged with them, punctuation down to parens, braces and separators.
- **`HighlightStyle`, as in CodeMirror.** `NewHighlightStyle` takes `TagStyle`
  specs (tags, color, bold, italic, underline, line-through) and resolves a
  token's style like `HighlightStyle.define`: the most specific styled tag
  wins, and a later spec for a tag replaces an earlier one. `Specs` gives
  back a style's specs, so a theme can extend another's with specs of its own.
- **Ported CodeMirror themes**: `OneDark` (`@codemirror/theme-one-dark`), and
  `Dracula`, `GitHubLight`, `GitHubDark` and `Nord` (`@uiw/codemirror-themes`),
  spec for spec. The demo takes `-theme`.

### Changed

- **Breaking: themes style tags, not classes.** `Theme.Syntax`, `TokenStyle` and
  `TokenClass` are gone. A theme of your own sets `Theme.Foreground` (what
  `Syntax[Plain].Color` was) and `Theme.Highlight`, a `*HighlightStyle` built
  from `TagStyle` specs naming `tags`. `DarkTheme` and `LightTheme` keep their
  colors, and a few tokens move with the finer tags: `self`/`this` take the
  keyword color, HTML entities the string color, exception names the type
  color, symbols and atoms the literal color; markdown headings and strong text
  are bold, emphasis italic, links underlined.

### Fixed

- **Resizing a view no longer redraws every row.** A `View`, `Split` or
  `DiffView` whose width changed every frame (a pane edge being dragged)
  composed every visible row again each frame; now a row is composed again
  only when the cells it shows change.

## [0.1.0] - 2026-10-10

### Added

- **A read-only code view for Gio.** `View` shows one document: monospace rows
  highlighted by chroma from the file name, a line-number gutter (or labels of
  your own), wrapping or sideways scrolling, and one image per visible row, so
  a full screen of code costs a handful of quads. Highlighting runs off the UI
  goroutine; `SetContent` shows a new document from its top and
  `UpdateContent` swaps in a new version of it without moving the scroll.
- **Side by side.** `Split` shows two documents row for row, scrolling down
  together and sideways each on its own, and it can wrap: a pair of lines takes
  the taller side's rows.
- **Decorations and widgets.** `LineDecoration` tints a line and its gutter
  cell, puts a strip down the gutter, rules a separator, hatches a filler row,
  tints spans of characters, and lights a whole hover group under the pointer.
  `LineWidget` puts a Gio widget on a line (after its text or at the right
  edge, optionally only while hovered); `BlockWidget` opens space between lines
  for any widget, such as a review thread.
- **Selection.** Drag, double and triple click, Shift+click, Ctrl+A and Ctrl+C,
  with tabs copied as written. From code: `Select`, `SelectLines`,
  `Selection`, `SelectedText` and `SelectedLines`, plus `ScrollToLine` with
  `AlignNearest`, `AlignCenter` and `AlignTop`, `LineCount` and `LineText`.
  A selection dragged past the view's edge scrolls it, faster the farther the
  pointer is beyond (sideways too when unwrapped), and keeps going on frames
  where the pointer does not move.
- **A diff view.** `DiffView` diffs two documents itself (`SetDocs`, line by
  line with Myers' algorithm) or shows a file of a `git diff` patch
  (`ParsePatch`, `SetPatch`). Unified, with an old | new line-number gutter,
  or side by side, with deletions and additions paired and fillers hatched.
  Unchanged runs collapse behind an expander (`Collapse`, or the gaps of the
  patch, filled through `DiffOptions.Expand` and `FillGap`, in parts if need
  be). Each chunk can be reverted through `DiffOptions.Revert`; `Chunks`,
  `SelectedRange` and `SelectedChanges` read the diff in file line numbers,
  and `SetBlocks` anchors widgets to file lines. `HighlightChanges` tints the
  words that changed within a line. The expander and revert affordances are
  drawn by the library and can be replaced.
- **Search.** `View.SetSearch` takes a `SearchQuery` (literal or regexp, case
  sensitivity, whole word) and highlights every match, beneath the text, in
  `Theme.SearchMatch`; the selected match is the current one, in
  `Theme.SearchCurrent`. `Matches` lists them and `FindNext` and `FindPrevious`
  select the one after or before the selection, wrapping, and scroll it into
  view, sideways too when lines do not wrap. A match never spans lines, and a
  bad regexp is the only error. `DiffView.SetSearch`, `Matches`, `FindNext`
  and `FindPrevious` do the same across its unified view and both sides of the
  split.
- **Themes.** `DarkTheme` and `LightTheme`, with the diff colors in
  `Theme.Diff`. A theme change shows on the next frame.
- **Fonts without fontconfig.** `New` resolves a family through fontconfig and
  fails with `ErrNoFontconfig` where it is missing; `NewFromFonts` (and
  `NewDiffViewFromFonts`) take the font files' contents instead. A character no
  font covers is drawn as a box.
- **Wide characters take two cells.** CJK, fullwidth forms and emoji are laid
  out on the monospace grid as a terminal lays them out, so the columns after
  them line up; a click on either half lands before or after the character,
  wrapping never splits one, and a gutter label's width counts them too.
  `Pos.Col` and `Span` still count runes.

[Unreleased]: https://github.com/lichdotdev/godemirror/compare/v0.7.1...HEAD
[0.7.1]: https://github.com/lichdotdev/godemirror/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/lichdotdev/godemirror/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/lichdotdev/godemirror/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/lichdotdev/godemirror/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/lichdotdev/godemirror/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/lichdotdev/godemirror/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/lichdotdev/godemirror/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/lichdotdev/godemirror/releases/tag/v0.1.0
