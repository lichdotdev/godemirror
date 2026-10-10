# godemirror

`godemirror` is a **read-only code view for [Gio](https://gioui.org)**: the diff viewer (unified and side by
side) and the file preview that [lich](https://github.com/omartelo/lich) needs for its native port, in place of
CodeMirror. It is built for other developers to use, not only for lich: docs, errors and defaults answer to a
stranger. Linux first, since fonts resolve through fontconfig.

**This file is instruction, not documentation.** Rules, gates and the workflow live here. What the library *is*
lives in the code, its godoc, `README.md` and `docs/`; never restate any of it here.

## Where things are written down

- **Deliberate limits and the traps they set**: `docs/ceilings.md`. Read it before touching rendering, the row
  caches, input, highlighting or the split. A change that sets a new trap or closes an old one moves that file
  in the same PR.
- **What it does and how to use it**: `README.md` and the package godoc.
- **Project memory**: the ai-memory project `default/godemirror` (pinned by `.ai-memory.toml`) holds the
  overview, the Gio pitfalls already measured and the roadmap. Query it before proposing a design.

## Rules of the codebase

- Go 1.27 and Gio v0.10. The dependency set is Gio, `golang.org/x/image` and chroma; a new one needs the
  maintainer's yes first.
- **The package stays app-free.** Nothing that belongs to one app enters it: lich's diff layout, its buttons,
  its strings and its theme live in the demo or in the app. The package takes data (`Content`) and Gio widgets
  (`LineWidget`, `BlockWidget`) and draws them.
- **Code text never goes through Gio's text stack.** Rows are images composed by `glyphs`; Gio text is only for
  what callers draw inside their own widgets.
- **A row-image cache key names everything the image holds**, or the cache is dropped when that thing changes.
  Two shipped bugs came from a key that did not (wrap toggled, window grown).
- **A caller's widget is laid out exactly once per frame.** Gio widgets read their events during layout; a
  second layout, for measuring or anything else, loses clicks.
- **Colors are what CSS would show.** Gio blends alpha in linear light, so a translucent color is mixed over
  the theme background in sRGB (`over`) before it is painted.
- Bug fixes start with a test that reproduces the bug, and the test is seen failing without the fix.

## Local Gate (before every commit / PR)

- `gofmt -l .` clean and `go vet ./...` clean.
- `go test ./...` green. Anything asynchronous (highlighting, `UpdateContent`) also passes
  `go test -count=50 -run <Test> ./...`.
- Changed what is drawn? Take a headless shot with the demo (`-shot`, with `-hover` or `-split` as needed) and
  look at it before calling it done. The maintainer then reviews it in a real window, and that review is the
  one to trust.
- Changed a limit or a trap? `docs/ceilings.md` moves in the same change.

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
