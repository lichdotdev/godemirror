// Package lsp is a client of the Language Server Protocol 3.17: it starts a
// language server, keeps it in step with the documents an Editor edits, and
// asks it for diagnostics, completions, hovers and definitions.
//
// Start runs a server from a command the caller gives (the package knows no
// server; the servers package is a catalog of them) and greets it;
// Connect does the same over any stream, a server already running or one in
// a test.
//
// Messages are JSON-RPC 2.0 framed by Content-Length headers, over the
// server's standard input and output. Positions are offered in UTF-8, the
// unit an Editor counts in; a server that only speaks UTF-16, the default
// of the protocol, gets UTF-16, and PositionEncoding converts columns.
package lsp
