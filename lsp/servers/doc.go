// Package servers is a catalog of language servers: for each one, the
// command that starts it, the vim filetypes it serves, the files that mark a
// project root, its default initialization options and settings, and how to
// install it. Lookup picks the candidates for a file by its name.
//
// The catalog only describes. It never runs a server and never runs an
// install: an Install is a recipe a caller may follow, with its own tools.
//
// The data is converted from nvim-lspconfig's lsp/*.lua configs and from
// mason-registry's registry.json, both Apache-2.0, at the versions named in
// catalog_generated.go; LICENSE.apache and NOTICE carry their terms. Where an
// lspconfig config computes a field in Lua, the catalog cannot carry it:
// the field's name goes into Server.Dynamic and its value is lost. That is
// every field that is or holds a function (cmd, root_dir, on_attach,
// before_init, handlers and the like), and every field whose value depends
// on the machine the config was evaluated on (a path under $HOME, a pid,
// nvim's version). A Server whose cmd is computed has no Command, but for
// ts_ls and jdtls, which carry the command lspconfig starts by default; its
// Install's Bins usually still name the executable. Fields lspconfig sets
// as plain data but this package has no place for (capabilities,
// workspace_required and others) are dropped.
//
// Filetypes are vim's: a file's name maps to one through nvim's own
// extension and filename tables, as nvim decides for a new, empty file of
// that name (so a .h file is cpp, and a .ts file typescript).
package servers

//go:generate node ../../internal/serversgen/gen.mjs
