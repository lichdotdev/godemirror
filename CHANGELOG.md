# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`Editor.CloseBrackets`** types brackets and quotes in pairs, as CodeMirror's `closeBrackets`: an opening
  one typed before a space, a closing bracket or the line's end inserts its closing one, typing the closing one
  steps over it, and Backspace in an empty pair deletes both. Off by default.
- **`Editor.IndentWithTab`** binds Tab and Shift+Tab, as CodeMirror's `indentWithTab`: Tab inserts an
  `IndentUnit` at each cursor, or indents the lines of a selection, and Shift+Tab takes a unit off the lines a
  range is on. Off by default, so Tab still moves focus.
- **`EditorStateConfig.BracketIndent`** indents by brackets, where CodeMirror would ask a language: Enter
  after an open `(`, `[` or `{` indents the new line one `IndentUnit` more (between `{}` the middle line), and
  a closing bracket typed first on a line takes one unit off. Off by default, so Enter stays as it was.
- **The `godemirror_nochroma` build tag** leaves out chroma's lexers, for a smaller WebAssembly build: files that
  no Lezer grammar or stream mode reads show plain. The site's demo drops from 22.4 MB to 16.7 MB (4.6 MB
  gzipped).

### Fixed

- **A dead key no longer throws the text and the caret elsewhere in a browser** (` on an ABNT2 keyboard,
  then a letter): Gio's browser text input took its whole textarea, the text the editor gave it around the
  caret, for the composed text, and inserted it again. In a browser the editor now gives it no text around
  the caret.
- **An edited line keeps its colors until highlighting catches up**, instead of showing uncolored for that
  time: the text around the change keeps its tags, and text typed inside a token takes that token's. The gap
  was invisible natively but flashed on every keystroke in a WebAssembly build.
- **Taking a completion that ends in a signature trigger opens the signature help**, as typing the trigger
  would: a snippet like `Since($1)` from gopls now shows `Since(t time.Time)` with its field selected.
- **The signature help no longer hides under the completion menu.** When the menu covers it, the signature
  moves to the other side of the cursor's row; when there is no room there, it is not drawn until the menu
  closes.
- **A wrapped line's first row no longer shows only its indentation** when the word after it is longer than
  the rest of the row: the word is split on that row instead.
- The completion menu no longer flickers while typing against a slow server: a word typed waits for a pause of
  120 ms before it asks (a trigger character or Ctrl+Space still asks at once), and the menu keeps its
  items, filtered by what was typed since, until the answer replaces them, the chosen item staying chosen when
  it is still offered. A quick burst of typing is one request.
- Signature help is asked again once typing pauses, not on every transaction, and stays shown until the answer.
- **A completion taken inside a snippet's field no longer ends the snippet** when the item is a snippet
  with no fields of its own, as gopls sends a variable: Tab still goes to the next field, or past the end.
- **An `Editor` asks for the text input when it is pressed or focused**, so typing reaches it in a browser
  (WebAssembly), on Android and on iOS, which open their text input only when asked. A read-only `View` still
  does not ask.

## [0.12.0] - 2026-11-26

### Changed

- **`SetScrollTop`, `ScrollToLine` and an `Editor`'s scroll into view now glide** to their target over 140 ms,
  eased out, instead of snapping, unless the jump is three rows or less: PageDown, Ctrl+End, `NextDiagnostic`
  and a definition in the same file slide there. Right after the call, `ScrollTop` still answers where the
  view is; `ScrollTarget` answers where it lands. Call `SnapScrollTop` for the old behaviour.
- **`OnScroll` hears only the user's scrolls** (wheel, touchpad, a selection dragged past an edge), no longer
  `SetScrollTop`, `ScrollToLine`, an `Editor`'s cursor kept in view, nor any frame of a glide: an app that
  mirrors the scroll elsewhere hears no echo of its own. A pane that followed every scroll through `OnScroll`
  reads `ScrollTop` after each `Layout` instead.
- **A wrapped row now hangs under its line's own indentation**, two cells in, up to half the width,
  in a `View`, an `Editor` and a `DiffView`, unified and split, instead of starting flush left. The
  indent is only drawn: selection, search, copy and `Pos` see the line's own text, and the gutter stays
  empty on continuation rows. A line breaks into more rows than before, and `DiffView.Height`
  counts them.
- **A focused `Editor` takes Ctrl+., F2, F8, Shift+F8, F12, Ctrl+F12, Shift+F12, Ctrl+Shift+F12 and Ctrl+Space**, for the diagnostics, the definition and the
  completions below; an app that binds them for itself reads them before the editor's `Layout`. F12 and
  Ctrl+Space do nothing while no `Definition` or `Complete` is set.
- **`Theme` has two more fields, `Diagnostics` and `Popup`**: a theme built field by field rather than copied
  from one of the package's draws its diagnostics and popups in the zero color until it sets them. The ported
  themes take CodeMirror's base colors for them (`@codemirror/lint`, `@codemirror/view`'s tooltip,
  `@codemirror/autocomplete`'s chosen item), and `OneDark` its own tooltip and chosen item.

### Added

- **`SnapScrollTop`, `ScrollTarget` and `ReduceMotion`** on `View` and `Editor`: scroll without the glide,
  read where a glide lands, and turn every glide off (Gio reports no platform preference for it).
- **Diagnostics in an `Editor`**: `SetDiagnostics` takes `Diagnostic`s (a byte range, a `Severity`, a message,
  its source and code), which follow later edits, and the editor underlines each with a wave in its
  severity's color, puts a dot in the gutter of its line (the worst one's), and shows the messages in a popup
  while the pointer rests on one. `NextDiagnostic` and `PrevDiagnostic` move the cursor to the next one, of a
  severity or any, and show it there; F8 and Shift+F8 run them.
- **Completions in an `Editor`**: set `Complete` and the editor asks it after a `CompletionTriggers`
  character, after two word characters, or on Ctrl+Space, off the UI goroutine, and shows the
  `CompletionItem`s in a menu under the cursor: a badge of the item's `CompletionKind`, the typed part in bold,
  the detail on the right, and the chosen item's documentation beside the menu, which `ResolveCompletion`
  may fill in. Up and Down choose, Enter and Tab take, Escape closes; typing on filters the items.
- **Hover and go to definition in an `Editor`**: `Hover` answers, off the UI goroutine, the Markdown that
  shows above a name the pointer rests on. With `Definition` set, Ctrl shows the
  name under the pointer as a link, and Ctrl+click or F12 go to its definition: in the document, the cursor
  moves; in another, `OnDefinition` gets its `Location`. `DefinitionHint` is the hint under a hover.
- **Type definition, implementation and references in an `Editor`**: `TypeDefinition` and `Implementation`
  (both `GoTo`, as `Definition` now is) go where they find as a definition goes, on `TypeDefinitionKey`
  (Ctrl+Shift+F12) and `ImplementationKey` (Ctrl+F12). `References` runs on `ReferencesKey` (Shift+F12):
  the references in the document are highlighted in the new `Theme.Occurrence` until an edit or Escape, and
  all of them go to `OnReferences` for the app to list. The keys are `Shortcut`s a caller may change. `lsp`
  asks the server's typeDefinition, implementation and references (`Client.TypeDefinition`,
  `Implementation`, `References`), and `Attach` sets the three hooks.
- **Popups render Markdown, their prose in a proportional font**: the hover and a completion's
  `Documentation` are Markdown (paragraphs, lists, headings, fenced code highlighted by its language, inline
  code, bold, italic, links as text), their prose set through Gio's text stack in the Go font and their code
  in the editor's. A popup grows to 560dp by 420dp and scrolls with the wheel past that.
- **Snippets in an `Editor`**: `InsertSnippet(from, to, snippet)` inserts LSP's snippet syntax (`$1`,
  `${1:name}`, `${1|a,b|}`, `$0`), its lines indented as the line it goes on, and selects its first field;
  Tab and Shift+Tab go from field to field, and `$0`, Escape or a selection out of it end it. A
  `CompletionItem` with `Snippet` set is inserted so, and `lsp` now tells servers it takes snippets.
- **Signature help in an `Editor`**: `SignatureHelp` tells the `Signature` of the call being typed after
  one of `SignatureTriggers`, and again after every transaction while it answers; the popup above the cursor
  shows its label as code, the active parameter bold, and its documentation, and closes when the hook
  answers none (past the call) or on Escape. `lsp` asks the server's signatureHelp
  (`Client.SignatureHelp`, `SignatureTriggers`), parameters given as substrings or offsets.
- **Rename in an `Editor`**: F2 opens an input over the name at the cursor; Enter asks `Rename` for the
  `DocumentEdit`s (`TextEdit`s by document), applies those to the editor's document as one transaction and
  hands the others to `OnWorkspaceEdit`; an error the hook returns shows at the symbol. `lsp` asks the
  server's rename (`Client.Rename`) and reads a `WorkspaceEdit` either way the server writes it.
- **Code actions in an `Editor`**: when the cursor reaches a line with a diagnostic the editor asks
  `CodeActions`, and a bulb in the gutter's marks cell, in place of the line's diagnostic dot and in its
  color, shows there are some; Ctrl+. or a click on the bulb lists them, and
  the one taken goes to `ApplyCodeAction`, whose edits are made as a rename's. `Editor.ApplyEdits` makes
  `DocumentEdit`s from elsewhere the same way, and `Diagnostic.Data` carries a source's own data. `lsp` asks
  the server's codeAction, resolves an action with no edit, runs its command (`Client.CodeActions`,
  `ResolveCodeAction`, `ExecuteCommand`) and applies the server's workspace/applyEdit to the documents
  attached.
- **`Document.Format(ctx)`** formats the whole document through the server (`Client.Format`), indented as
  the new `EditorState.IndentUnit`, and makes its edits one transaction that one Undo takes back.
  Formatting on save stays the caller's choice. The demo's `-format` runs it.
- **`Document.Symbols(ctx)`**, the document's outline as the server lists it (`documentSymbol`): a tree of
  `Symbol`s (name, detail, `SymbolKind`, range and name's range in the editor's units, children), or a flat
  list from a server that does not nest them, for the app to draw. The demo's `-symbols` prints it.
- **The mouse in an `Editor`'s popups**: a click on a completion takes it, the wheel scrolls a long menu, the
  pointer over the hover keeps it open, and a click in the code closes the menu and the hover. The code
  under a popup no longer sees the pointer over it.
- **`Editor.Post(f)`**, safe from any goroutine: runs `f` at the start of the editor's next `Layout`.

- **`lsp`, a Language Server Protocol client**: `lsp.Start` runs a server from a command and root
  directory the caller gives, `lsp.Connect` speaks to one over any stream, and the `Client` keeps
  documents in step (`DidOpen`, `DidChange`, incremental or full as the server asks, `DidSave`,
  `DidClose`) and asks for `Completion`, `ResolveCompletion`, `Hover` and `Definition`;
  `WatchDiagnostics` hands over what the server publishes. It offers UTF-8 positions and falls back
  to UTF-16, which `PositionEncoding` converts. Standard library only.
- **`lsp.Attach(client, editor, uri, opts)`**, the editor and the server bound: the document opens on the
  server, every edit follows (incrementally or whole, as the server syncs), published diagnostics show in
  the editor mapped through the edits made since, and the editor's completions, hover and definition ask
  the server.
  `Document.Save` and `Document.Close` send didSave and didClose. The demo's `-lsp gopls` runs it.
- **`lsp/servers`, an optional catalog of 428 language servers** converted from nvim-lspconfig and
  mason-registry (Apache-2.0): each server's command, filetypes, root markers, default init options
  and settings, and a plain-data install recipe (npm, pypi, golang, cargo, gem, or a GitHub release
  asset per platform) for 252 of them. `Lookup(path)` returns a file's candidates with the common
  server first. The catalog never runs a server nor an install.
- **`servers.FileType(path)` and `servers.LanguageID(filetype)`**: the vim filetype `Lookup` reads a file
  as, and the LSP languageId of a filetype for `lsp.Options.LanguageID`, LSP's name where the two differ
  (`sh` is `shellscript`, `cs` `csharp`, `objc` `objective-c`).
- **`DiffOptions.UnifiedGutter`**, how a unified diff numbers its rows: `TwoColumnGutter`, the
  default, shows the old and the new number, and `OneColumnGutter` shows one, the line's number in
  the modified file or, for a deleted line, in the original, in about half the width.
- **`GutterCell`, a `Placement` that draws a `LineWidget` over its line's gutter cell**, on the
  line's first row and in place of its label, which is not drawn while the widget is.
  `DiffOptions.RevertPlacement` takes it for the revert with `RevertInGutter`; the default,
  `RevertAtRightEdge`, is what a `DiffView` always did. In the gutter the default revert is the
  compact "↩", and `RevertWidget` still replaces it.
- **`DiffView.Height(gtx, width)`**, the height of the diff's rows laid out `width` pixels wide, in
  the mode the view is in (`Split`, `Wrap`), to size a `DiffView` from outside without laying it
  out. With `Wrap` it counts the rows each line wraps into; `BlockWidget`s are not counted.
- **`Editor.OnKey`, every key and typed text before the editor acts on it**, as a keydown
  handler of CodeMirror's `EditorView.domEventHandlers`: it gets each `key.Event`, pressed and
  released, and each `key.EditEvent`, and returning true keeps the editor from acting (no
  keymap command, no Ctrl+C, X or V, no typed text). While it is set the focused editor takes
  every key, Tab and Shift+Tab included, which then stop moving focus. With it unset nothing
  changes.
- **`Editor.FormatNumber`**, as `lineNumbers`' `formatNumber`: spells each gutter number from the state on
  screen, so a caller can number lines relative to the cursor.
- **`Editor.HasFocus(gtx)`**, as CodeMirror's `hasFocus`: whether the editor has the keyboard,
  answered before the editor's Layout in a frame as well as after.
- **`Editor.CoordsAtPos(pos, side)`**, as CodeMirror's `coordsAtPos`: the zero-width rectangle,
  a row high, where a position shows, in pixels from the editor's top left as the last Layout
  placed it, wrapping and scrolling counted; false before a Layout and for a position outside
  the document or inside a character.

## [0.11.0] - 2026-11-23

### Added

- **`Highlight`, a text's highlighted runs for a caller that draws the text itself**, a code
  block in Markdown, say: the language picked by the filename as `Content.Filename` picks it, and
  each line's runs in the `TagStyle` a `View` with the same `Theme` draws them in, tabs expanded
  to the view's stops. It runs on the calling goroutine.

## [0.10.0] - 2026-11-19

### Added

- **`View.Geometry` and `Editor.Geometry`**, where the last Layout put the gutter and the text
  across: `GutterWidth`, `TextX` and `CellWidth`, in pixels, so a caller can draw beside the gutter
  or past a line's text.

## [0.9.0] - 2026-11-15

### Added

- **`Editor`, an opt-in editable view**, built with `NewEditor` or `NewEditorFromFonts` and
  holding an `EditorState`, a port of `@codemirror/state` 6.7.6's document and selection.
  `Dispatch(TransactionSpec)` replaces ranges of the document and sets the selection as one
  transaction; `OnUpdate` reports each frame's transactions after the frame, as CodeMirror's
  `updateListener`. `ChangeSet` (with `MapPos`, `MapPosTrack` and `MapMode`), `EditorSelection`
  and `SelectionRange` follow CodeMirror's: a selection holds one or more ranges, sorted and merged,
  when `EditorStateConfig.AllowMultipleSelections` is set. `ChangeByRange` and
  `ReplaceSelection` change every range at once, and `LineSeparator` keeps a CRLF file CRLF.
  Positions are UTF-8 byte offsets, where a `View`'s `Pos` counts runes. An edit rebuilds only the lines it
  touched on screen. `View`, `Split` and
  `DiffView` stay read-only and unchanged.
- **An `Editor` takes the keyboard and the mouse as CodeMirror does**, ported from
  `@codemirror/commands` 6.11.1 and `@codemirror/view` 6.43.15: typing and input methods at
  every cursor; `standardKeymap`'s motions and deletions by character, group, line, page, line
  boundary and document, with Shift to select; Enter keeping the line's indentation in the new
  `EditorStateConfig.IndentUnit`; Ctrl+A; copy, cut and paste, a whole line when nothing is
  selected, a line per cursor when the counts match; CodeMirror's mouse selection, Ctrl+click
  adding a range; and a blinking caret per range while focused, `Focus` to give it the keyboard.
  `TransactionSpec.ScrollIntoView` scrolls the main cursor into view, and `SelectionRange` has
  `GoalColumn`.
- **`Theme.Cursor`** colors an `Editor`'s caret: each ported theme takes its source's, and
  `DarkTheme` and `LightTheme` their foreground.
- **Several cursors at once in an `Editor`**, from `@codemirror/commands` 6.11.1's
  `defaultKeymap`: Ctrl+Alt+Up and Down add a cursor on the line above or below each one
  (`addCursorAbove`, `addCursorBelow`), and Escape goes back to the main cursor, or a range to
  its head (`simplifySelection`). `Editor.RectangularSelection` makes an Alt drag select a
  rectangle, a range per line, and `Editor.CrosshairCursor` shows a crosshair while Alt is held,
  as `@codemirror/view` 6.43.15's `rectangularSelection` and `crosshairCursor` do.
- **Ctrl+D and Ctrl+Shift+L in an `Editor`**, `@codemirror/search` 6.7.2's
  `selectNextOccurrence` and `selectSelectionMatches`: Ctrl+D selects the word around each cursor,
  then adds the next occurrence of the selection, a whole word when the selection is one; Ctrl+Shift+L
  selects every occurrence of the selected text. Both match through NFKD normalization, as its
  `SearchCursor` does: the two forms of `é` find each other, a ligature its letters, and an `e`
  the `e` of an `é`, which it selects whole. godemirror depends on `golang.org/x/text` for it.
- **Undo and redo in an `Editor`**, `@codemirror/commands` 6.11.1's history: changes typed or
  deleted close together are one undo event; `Undo`, `Redo`, `UndoDepth` and `RedoDepth`, and
  Ctrl+Z, Ctrl+Y and Ctrl+Shift+Z, Ctrl+U and Alt+U to undo and redo the selection alone.
  `TransactionSpec.IsolateHistory` makes a dispatched change an undo event of its own,
  `SkipHistory` keeps it out of the history, and `Time` sets when it happened; `Transaction`
  carries all three.
- **Decorations on an `Editor` that follow edits**: `LineDecorationAt` puts a `LineDecoration` on
  the line starting at a position, and `LineWidgetAt` a `LineWidget` on the line of a position,
  mapped through every change as `@codemirror/state` 6.7.6's `RangeSet.map` maps
  `@codemirror/view`'s line and widget decorations: a line decoration goes with the line break
  before its line, a widget with a deletion across it. They start in
  `EditorStateConfig.Decorations`, are replaced by `TransactionSpec.Decorations`, and are read
  back with `EditorState.Decorations`.
- **Scroll and line geometry on a `View` and an `Editor`**, as CodeMirror's `scrollDOM.scrollTop`,
  `lineBlockAt`, `lineBlockAtHeight` and `contentHeight` answer them: `ScrollTop` and
  `SetScrollTop`; `LineBlock` (`LineBlockAt` by position on an `Editor`), the top and bottom of a
  line's rows in the last frame; `LineAtHeight` (`LineBlockAtHeight`); `ContentHeight`; and
  `OnScroll`, called after a frame that left the view scrolled elsewhere, so that views beside it
  can follow it. Between a new content and the next frame, the geometry is an estimate of a row a
  line, as CodeMirror estimates lines it has not measured.

## [0.8.1] - 2026-10-23

### Fixed

- **A `DiffView` no longer colors the rows after a gap with what the rows
  before it left open.** A block comment opened above a gap and closed in the
  lines it leaves out used to run on as a comment through the following hunks
  until some later `*/`; highlighting now starts over at each gap.

## [0.8.0] - 2026-10-19

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

[Unreleased]: https://github.com/lichdotdev/godemirror/compare/v0.12.0...HEAD
[0.12.0]: https://github.com/lichdotdev/godemirror/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/lichdotdev/godemirror/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/lichdotdev/godemirror/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/lichdotdev/godemirror/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/lichdotdev/godemirror/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/lichdotdev/godemirror/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/lichdotdev/godemirror/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/lichdotdev/godemirror/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/lichdotdev/godemirror/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/lichdotdev/godemirror/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/lichdotdev/godemirror/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/lichdotdev/godemirror/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/lichdotdev/godemirror/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/lichdotdev/godemirror/releases/tag/v0.1.0
