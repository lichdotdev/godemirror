# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/lichdotdev/godemirror/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/lichdotdev/godemirror/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/lichdotdev/godemirror/releases/tag/v0.1.0
