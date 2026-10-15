# Contributing

Bugs and ideas are welcome, and so are patches. This file is the short version;
`CLAUDE.md` holds the invariants, and [`docs/ceilings.md`](docs/ceilings.md)
the deliberate limits and the traps they set.

## Reporting

- **Something is broken**: the [bug form](https://github.com/lichdotdev/godemirror/issues/new?template=bug.yml).
  A few lines of Go that show it are worth more than a description.
- **Something is missing**: the [feature form](https://github.com/lichdotdev/godemirror/issues/new?template=feature.yml).
  Describe the problem first; the API you have in mind is optional, and a
  well-described problem often has a better answer than either of us reaches
  for first.
- **A security problem**: not an issue. See [SECURITY.md](SECURITY.md).

## Getting it running

Prerequisites: **Go 1.27.0+**, and, for the demo and the race detector, a C
toolchain and the system libraries Gio needs on your platform
([Gio's install guide](https://gioui.org/doc/install)). On Debian/Ubuntu:

```bash
sudo apt install gcc pkg-config libwayland-dev libx11-dev libx11-xcb-dev \
  libxkbcommon-x11-dev libgles2-mesa-dev libegl1-mesa-dev libffi-dev \
  libxcursor-dev libvulkan-dev fontconfig fonts-dejavu-core
```

The tests resolve the `monospace` family through fontconfig, so they need
fontconfig and at least one monospace font installed.

```bash
go build ./...
go test ./...
```

The demo shows what the library draws, in a window or headless:

```bash
go run ./cmd/godemirror-demo                                  # a file in a View
go run ./cmd/godemirror-demo -old a.go -file b.go -split      # a DiffView of two files
git diff > /tmp/change.patch
go run ./cmd/godemirror-demo -diff /tmp/change.patch -new path/to/file
go run ./cmd/godemirror-demo -old a.go -file b.go -shot out.png   # headless PNG of the last frame
```

`go run ./cmd/godemirror-demo -h` lists every flag.

## Lezer grammars

`internal/lezer` runs CodeMirror's Lezer grammars from tables generated out of
their npm packages, and checks itself against goldens the real Lezer writes.
Both come from scripts in `internal/lezer/gen`, run by `go generate`; they need
**node**, at development time only, and the packages installed outside the
repository, at the versions the grammar's `go:generate` lines and
`docs/ceilings.md` name:

```bash
cd "$(mktemp -d)" && npm init -y && npm install @lezer/lr@1.4.11 @lezer/common@1.5.3 \
  @lezer/highlight@1.2.5 @lezer/java@1.1.5
LEZER_NODE_MODULES="$PWD/node_modules" go generate ./internal/lezer/java
```

That rewrites the grammar's `spec_generated.go` (its tables, styles and term
ids) and the goldens under its `testdata`. Never edit either by hand: a
fixture is added by dropping it into `testdata` and generating again. A
grammar's JavaScript (external tokenizers, specializers, a context tracker) is
ported by hand next to it, from the same package version.

## Before you open a pull request

Run the same gate CI runs:

```bash
gofmt -l .              # must print nothing (fix: gofmt -w .)
go vet ./...
go test -race ./...
```

Then:

- **Touched anything asynchronous** (highlighting, `UpdateContent`, a
  `DiffView` update)? Run its tests 50 times: `go test -count=50 -run TestX ./...`.
- **Changed what is drawn?** Take a headless shot with the demo (`-shot`, with
  `-hover` or `-split` as needed) and look at it. Attach it to the pull request.
- **Changed a limit, or set a new trap?** `docs/ceilings.md` moves in the same
  pull request.
- **Changed anything a caller can see** (exported API, behaviour, what is drawn,
  a fixed bug)? Add its `CHANGELOG.md` `[Unreleased]` entry in the same pull
  request.

A change to the API is best proposed in an issue before it is written: say
what a caller does today, what they would write instead, and what it costs.
New dependencies need the maintainer's agreement first; the library depends on
Gio, `golang.org/x/image` and chroma only.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org)
(`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, `perf:`, `ci:`).

## Three rules worth stating outright

**A bug fix starts with a test that reproduces it**, seen failing without the
fix. Tests drive the view through Gio's own `input.Router` (the `harness` in
the tests) wherever the behaviour can be reached that way.

**Tests answer to the contract, never the other way round.** Never weaken, skip
or delete a test to buy a green run. A test changes for exactly two reasons:
the contract changed, or it asserted something the contract never promised.
Say which one in the pull request.

**A flake is a bug with a root cause.** Don't re-run CI until it goes green.
Reproduce it (`go test -count=200 -run TestX ./...`), fix the cause, and say
what the failure rate was before and after.
