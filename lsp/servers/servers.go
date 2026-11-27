package servers

import (
	"cmp"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
)

// Server is one language server, named as nvim-lspconfig names it.
type Server struct {
	Name string
	// Command is the argv that starts the server on stdio, nil when
	// lspconfig computes it (then "cmd" is in Dynamic). ts_ls and jdtls,
	// whose lspconfig cmd is a function, carry the command it starts by
	// default, and keep "cmd" in Dynamic.
	Command   []string
	FileTypes []string
	// RootMarkers are file names that mark a project root, nearest first.
	// Each inner slice is one priority level whose names weigh the same;
	// lspconfig's flat list becomes one name per level.
	RootMarkers [][]string
	// InitOptions and Settings are lspconfig's defaults as JSON, nil when
	// it sets none: initializationOptions, and workspace/configuration's
	// answer.
	InitOptions json.RawMessage
	Settings    json.RawMessage
	// Dynamic names, sorted, the lspconfig fields computed in Lua, whose
	// values the catalog cannot carry.
	Dynamic []string
	// Install is nil when mason-registry has no recipe this package reads.
	Install *Install
}

// Kind is how a package installs, a purl type.
type Kind string

const (
	KindNpm    Kind = "npm"
	KindPypi   Kind = "pypi"
	KindGolang Kind = "golang"
	KindCargo  Kind = "cargo"
	KindGem    Kind = "gem"
	// KindGithub is a prebuilt release asset per platform, in Assets.
	KindGithub Kind = "github"
)

// Install is mason-registry's recipe for a server's package, its latest
// version only.
type Install struct {
	// Mason is the package's mason-registry name.
	Mason string
	Kind  Kind
	// Package is what the package manager installs: an npm or pypi name, a
	// Go package path (go install Package@Version), a crate, a gem, or a
	// GitHub owner/repo for KindGithub.
	Package string
	// Version is the exact version, or the GitHub release tag.
	Version string
	// Qualifiers are the purl's, verbatim: repository_url, rev, features,
	// extra and the like.
	Qualifiers map[string]string
	// ExtraPackages are installed next to Package, as name@version
	// (typescript for typescript-language-server).
	ExtraPackages []string
	// Bins are the executables the package provides.
	Bins []string
	// Assets are KindGithub's, one per target.
	Assets []Asset
}

// Asset is one platform's GitHub release download.
type Asset struct {
	// Target is mason's platform name: linux_x64, linux_x64_gnu,
	// linux_arm64_musl, darwin_arm64, win_x64, or a family (linux, unix).
	Target string
	// Files are release asset names, downloadable at
	// https://github.com/<Package>/releases/download/<Version>/<file>. A
	// name may end in ":dir/", mason's syntax for where it unpacks.
	Files []string
	// Bins maps each executable to its path among the unpacked files. A
	// path may carry mason's launcher prefix (exec:, java-jar:, node:,
	// python:, dotnet:) naming what runs it.
	Bins map[string]string
}

// languageIDs are the LSP languageIds of the vim filetypes that name a
// language otherwise, from the table of language identifiers in LSP 3.17's
// textDocument item (vim's tex is LaTeX, its plaintex plain TeX).
var languageIDs = map[string]string{
	"sh": "shellscript", "bash": "shellscript", "cs": "csharp", "objc": "objective-c",
	"objcpp": "objective-cpp", "make": "makefile", "gitcommit": "git-commit", "gitrebase": "git-rebase",
	"text": "plaintext", "ps1": "powershell", "raku": "raku", "perl6": "raku", "pug": "jade",
	"dosbatch": "bat", "coffee": "coffeescript", "tex": "latex", "plaintex": "tex",
}

// LanguageID is the LSP languageId of a vim filetype, as lsp.Options takes
// it: the filetype's own name for most, LSP's where the two differ ("sh" is
// "shellscript", "cs" "csharp", "objc" "objective-c"); "" for "".
func LanguageID(filetype string) string {
	if id, ok := languageIDs[filetype]; ok {
		return id
	}
	return filetype
}

// preferred puts the common server for its language ahead of the others.
var preferred = []string{
	"gopls",
	"ts_ls",
	"basedpyright",
	"pyright",
	"rust_analyzer",
	"clangd",
	"lua_ls",
	"jdtls",
}

// defaultCommands are what lspconfig's cmd function starts for a server the
// preference list puts first: its ts_ls prefers a typescript-language-server
// under the root's node_modules, and its jdtls adds -configuration and
// -data directories under Neovim's cache.
var defaultCommands = map[string][]string{
	"ts_ls": {"typescript-language-server", "--stdio"},
	"jdtls": {"jdtls"},
}

var (
	servers    = withDefaultCommands(catalog)
	byFileType = indexByFileType(servers)
)

func withDefaultCommands(all []Server) []Server {
	out := slices.Clone(all)
	for i, s := range out {
		if c, ok := defaultCommands[s.Name]; ok && s.Command == nil {
			out[i].Command = c
		}
	}
	return out
}

// All returns every server in the catalog, by name.
func All() []Server {
	return slices.Clone(servers)
}

// Lookup returns the servers for the file at path, nil when its name maps
// to no filetype a server claims. The common server for the language comes
// first (gopls for Go, ts_ls for TypeScript and JavaScript, basedpyright
// then pyright for Python, rust_analyzer, clangd for C and C++, lua_ls,
// jdtls), then the servers with a Command, then those whose command
// lspconfig computes; each group by name.
func Lookup(path string) []Server {
	return slices.Clone(byFileType[FileType(path)])
}

// FileType is the vim filetype of the file at path, by its name or
// extension, as Lookup reads it: "" when its name maps to no filetype a
// server in the catalog claims.
func FileType(path string) string {
	base := filepath.Base(path)
	if ft, ok := filenameFileTypes[base]; ok {
		return ft
	}
	ext := filepath.Ext(base)
	if ext == "" {
		return ""
	}
	return extensionFileTypes[strings.TrimPrefix(ext, ".")]
}

func indexByFileType(all []Server) map[string][]Server {
	index := map[string][]Server{}
	for _, s := range all {
		for _, ft := range s.FileTypes {
			index[ft] = append(index[ft], s)
		}
	}
	for _, list := range index {
		slices.SortStableFunc(list, compareServers)
	}
	return index
}

func compareServers(a, b Server) int {
	return cmp.Or(
		cmp.Compare(rank(a.Name), rank(b.Name)),
		cmp.Compare(computed(a), computed(b)),
		strings.Compare(a.Name, b.Name),
	)
}

func rank(name string) int {
	if i := slices.Index(preferred, name); i >= 0 {
		return i
	}
	return len(preferred)
}

func computed(s Server) int {
	if len(s.Command) == 0 {
		return 1
	}
	return 0
}
