# serversgen

Writes `lsp/servers/catalog_generated.go` and `lsp/servers/filetypes_generated.go`.

```sh
go generate ./lsp/servers   # or: node internal/serversgen/gen.mjs
```

Needs `node`, `git`, `unzip`, `nvim` (0.11 or later, for `lsp/*.lua`) and `gofmt` on `PATH`, and the network:
it fetches nvim-lspconfig at `LSPCONFIG_COMMIT` and mason-registry's `registry.json.zip` from the
`MASON_TAG` release. To move a pin, edit the constant at the top of `gen.mjs` and run it again. The
filetype tables and the check for machine-dependent values come from the `nvim` that runs, whose version
the generated header names.

`dump.lua` runs under `nvim -l` twice, each run in a bare environment with its own `$HOME`; a field
that is or holds a function, or that differs between the runs or carries a path or version of the
machine, goes into `Dynamic` instead of the catalog. The run prints the counts and every mason package
it could not turn into a recipe (unsupported purl types, GitHub sources built from source, templates
it does not expand).
