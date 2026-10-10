# Security policy

## Reporting a vulnerability

Report it privately through GitHub:
**[Security > Report a vulnerability](https://github.com/lichdotdev/godemirror/security/advisories/new)**.
That opens a private advisory only you and the maintainer can read.

Please don't open a public issue for a security problem: the issue tracker is
world-readable, and a working report there is a working exploit for everyone
else too.

Include what you did, what happened, and the version (the tag, or the commit
your `go.mod` pins). A proof of concept helps; a patch is welcome but never
expected.

## Supported versions

The latest release only. A fix ships as a new version rather than as a
backport.

## What the attack surface actually is

godemirror is a library inside your application. It opens no port and sends
nothing anywhere, so the interesting boundaries are the inputs it reads:

- **Text and patches.** `Content.Text`, `SetDocs` and `ParsePatch` take text
  that may come from anywhere. A crafted input that crashes the process, hangs
  it, or makes it use memory out of proportion to the input is a
  vulnerability.
- **Font files.** `NewFromFonts` parses font data it is handed, and `New` loads
  the files fontconfig names. The same applies: a crafted font that crashes or
  hangs the process belongs here.
- **fontconfig.** `New` runs `fc-match` and `fc-list` with fixed arguments,
  never through a shell. Getting them, or anything else, run with arguments of
  an attacker's choosing is a vulnerability.

## What is not a vulnerability

- **A slow diff of two huge, unrelated documents.** Myers' diff costs
  O((N+M)·D), and each stretch is capped at 10,000 edits; past that it is shown
  as one chunk. Documented in [`docs/ceilings.md`](docs/ceilings.md).
- **Running the fontconfig on your PATH.** `New` trusts the `fc-match` and
  `fc-list` it finds, as any program that resolves fonts does.
