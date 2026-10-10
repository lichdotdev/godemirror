## What and why

<!-- What a caller gets from this change, and the problem it solves. Link the issue if there is one. -->

## Checklist

- [ ] `gofmt -l .` prints nothing, `go vet ./...` is clean, `go test -race ./...` passes.
- [ ] Asynchronous code touched? Its tests pass with `-count=50`.
- [ ] A bug fix starts with a test that reproduces it, seen failing without the fix.
- [ ] A test changed? Say why: the contract changed, or it asserted something the contract never promised.
- [ ] Changed what is drawn? A headless shot from the demo is attached.
- [ ] Changed a limit or set a trap? `docs/ceilings.md` moved with it.
- [ ] Changed anything a caller can see? `CHANGELOG.md` has its `[Unreleased]` entry.
