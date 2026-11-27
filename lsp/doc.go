// Package lsp is a client of the Language Server Protocol 3.17 for
// godemirror's Editor: it starts a language server, keeps it in step with
// the documents an Editor edits, and fills the editor's diagnostics,
// completions, hovers, signatures, definitions, references, renames and code
// actions from it.
//
// Start runs a server from a command the caller gives (the package knows no
// server; the servers package is a catalog of them) and greets it;
// Connect does the same over any stream, a server already running or one in
// a test. Attach then binds an Editor to a document of the server: it opens
// the document, sends every edit, shows the server's diagnostics, and sets
// the editor's hooks to ask the server for completions, hovers, signatures,
// definitions, implementations, references, renames and code actions; the
// Document it returns formats the document and lists its symbols.
//
// An Editor on a Go file, with gopls:
//
//	client, err := lsp.Start(ctx, lsp.Config{Command: []string{"gopls"}, RootDir: root})
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//	doc, err := lsp.Attach(client, editor, lsp.FileURI(path), lsp.Options{
//		LanguageID: "go",
//		Invalidate: window.Invalidate, // diagnostics come between frames
//	})
//	if err != nil {
//		return err
//	}
//	defer doc.Close()
//
// cmd/examples/lsp is the whole program, the server picked from the
// lsp/servers catalog.
//
// Messages are JSON-RPC 2.0 framed by Content-Length headers, over the
// server's standard input and output. Positions are offered in UTF-8, the
// unit an Editor counts in; a server that only speaks UTF-16, the default
// of the protocol, gets UTF-16, and Attach converts every position.
package lsp
