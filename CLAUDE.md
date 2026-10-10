# godemirror

`godemirror` is an **open-source Go library that brings CodeMirror's read-only experience to native Go**, on
[Gio](https://gioui.org): the code view (`View`, `Split`) and the diff view (`DiffView`, unified and side by
side). That is the goal every decision answers to: when in doubt, ask what CodeMirror gives a caller and how a Go
caller would want it. [lich](https://github.com/omartelo/lich) is its first user, not its purpose: docs, errors
and defaults answer to a stranger, and nothing about lich belongs in the public docs. Linux first, since fonts
resolve through fontconfig.

**This file is instruction, not documentation.** Rules, gates and the workflow live here. What the library *is*
lives in the code, its godoc, `README.md` and `docs/`; never restate any of it here.

## Where things are written down

- **Deliberate limits and the traps they set**: `docs/ceilings.md`. Read it before touching rendering, the row
  caches, input, highlighting or the split. A change that sets a new trap or closes an old one moves that file
  in the same PR.
- **What it does and how to use it**: `README.md` and the package godoc.
- **What changed for a caller**: `CHANGELOG.md`, Keep a Changelog.
- **How an outside contributor works**: `CONTRIBUTING.md`. It restates the gate below for people who never read
  this file; when the gate changes, both move.
- **Project memory**: the ai-memory project `default/godemirror` (pinned by `.ai-memory.toml`) holds the
  overview, the Gio pitfalls already measured and the roadmap. Query it before proposing a design.

## Rules of the codebase

- Go 1.27 and Gio v0.10. The `go` directive in `go.mod` is the version CI installs (`go-version-file`), so it
  moves only on purpose. The dependency set is Gio, `golang.org/x/image` and chroma; a new one needs the
  maintainer's yes first.
- **The package stays app-free.** Nothing that belongs to one app enters it: an app's layout, its buttons, its
  strings and its theme live in the app. The package takes data (`Content`) and Gio widgets
  (`LineWidget`, `BlockWidget`) and draws them.
- **Code text never goes through Gio's text stack.** Rows are images composed by `glyphs`; Gio text is only for
  what callers draw inside their own widgets.
- **A row-image cache key names everything the image holds**, or the cache is dropped when that thing changes.
  Three shipped bugs came from a key that did not (wrap toggled, window grown, theme changed).
- **A caller's widget is laid out exactly once per frame.** Gio widgets read their events during layout; a
  second layout, for measuring or anything else, loses clicks. A `DiffBlock` goes to one side of a split, never
  both.
- **A `DiffView` calls its caller back after the frame**, never while a view paints: a callback may replace the
  content it was drawn from.
- **Colors are what CSS would show.** Gio blends alpha in linear light, so a translucent color is mixed over
  the theme background in sRGB (`over`) before it is painted.
- Bug fixes start with a test that reproduces the bug, and the test is seen failing without the fix.

## Local Gate (before every commit / PR)

- `gofmt -l .` clean and `go vet ./...` clean.
- `go test ./...` green. Anything asynchronous (highlighting, `UpdateContent`) also passes
  `go test -count=50 -run <Test> ./...`.
- Changed what is drawn? Take a headless shot with the demo (`-shot`, with `-old`, `-hover` or `-split` as
  needed) and look at it before calling it done. The maintainer then reviews it in a real window, and that
  review is the one to trust.
- Changed a limit or a trap? `docs/ceilings.md` moves in the same change.
- Changed anything a caller can see (exported API, behaviour, what is drawn, a fixed bug)? Its `CHANGELOG.md`
  `[Unreleased]` entry lands in the same commit, under the right group (Added, Changed, Deprecated, Removed,
  Fixed, Security). An entry written later is an entry that missed its release.
- Before a release, the gate also runs with `-race` (`go test -race ./...`), as CI does.

## Commits and the downstream pin

- Until the maintainer says otherwise, work lands as local commits on `main`: one commit per finished step (or
  per batch that lands together), with the gate green, and a Conventional Commits message in English (`feat:`,
  `fix:`, `docs:`, `refactor:`, `test:`, `ci:`, `chore:`). Unfinished work stays uncommitted. **Never push**, and
  never create the GitHub repository, until the maintainer says so.
- lich's spike builds against a pinned worktree of this repository (`../godemirror-pin`), at a commit the session
  driving it chooses. Report every commit's hash so it can be pinned, and never touch a worktree you did not
  create.

## Hard Invariants

Non-negotiable rules. A violation means the work is not done.

1. **Tests answer to the contract, never the other way round.** Never weaken, skip, delete or rewrite a test to
   buy a green run. A test may change for exactly two reasons: the contract changed, or the test asserted
   something the contract never promised. Name which one in the diff.
2. **A flake is a bug, and it has a root cause.** Reproduce it (`go test -count=200 -run TestX ./...`), fix the
   cause, and measure the failure rate before and after.
3. **Clean code.** Small focused functions (under 50 lines); cohesive files (200 to 400 lines typical, 800 max);
   no nesting deeper than 4 levels; comments only for the *why*; errors handled explicitly, never swallowed; no
   magic values.
4. **Tests go through the public behaviour.** Drive the view with Gio's own `input.Router` (the `harness` in
   the tests), not by poking fields, wherever the behaviour can be reached that way.

## Releases

The version is the git tag, `vX.Y.Z`, as Go modules read it: nothing in the code or in `go.mod` carries a
version, so never write one there. Before tagging:

- [ ] Local gate green, with `-race`.
- [ ] Cut the `CHANGELOG.md` section the Keep a Changelog way: `[Unreleased]` stays at the top, emptied, and its
      entries move under a new `## [X.Y.Z] - YYYY-MM-DD` heading (bracketed, ISO date, no `v`). Keep the
      groups in their canonical order (Added, Changed, Deprecated, Removed, Fixed, Security), and refresh the
      links at the bottom: `[X.Y.Z]` compares the previous tag with `vX.Y.Z` (the first release links to its
      tag), and `[Unreleased]` compares `vX.Y.Z` with `HEAD`.
- [ ] Before `v1.0.0` a minor version may break the API; say so in the section with a `### Changed` entry that
      names what a caller must change.
- [ ] Push the `vX.Y.Z` tag. pkg.go.dev picks the version up from the tag.
