package servers

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

func names(list []Server) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.Name
	}
	return out
}

func find(t *testing.T, name string) Server {
	t.Helper()
	i := slices.IndexFunc(All(), func(s Server) bool { return s.Name == name })
	if i < 0 {
		t.Fatalf("no server %q in the catalog", name)
	}
	return All()[i]
}

func TestLookupPutsTheCommonServerFirst(t *testing.T) {
	for path, want := range map[string][]string{
		"main.go":       {"gopls"},
		"x.ts":          {"ts_ls"},
		"x.tsx":         {"ts_ls"},
		"src/a.py":      {"basedpyright", "pyright"},
		"lib.rs":        {"rust_analyzer"},
		"a.c":           {"clangd"},
		"a.lua":         {"lua_ls"},
		"/abs/A.java":   {"jdtls"},
		"x.js":          {"ts_ls"},
		"component.jsx": {"ts_ls"},
	} {
		got := names(Lookup(path))
		if len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
			t.Errorf("Lookup(%q) = %v, want it to start with %v", path, got, want)
		}
	}
}

// Past the preferred servers, those with a Command come before those whose
// command is computed, each group by name.
func TestLookupOrdersTheRestByCommandThenName(t *testing.T) {
	got := Lookup("a.py")[2:]
	for i := 1; i < len(got); i++ {
		a, b := got[i-1], got[i]
		if computed(a) > computed(b) || computed(a) == computed(b) && a.Name > b.Name {
			t.Fatalf("Lookup(a.py) has %s before %s", a.Name, b.Name)
		}
	}
}

func TestLookupMatchesWholeFileNames(t *testing.T) {
	if got := names(Lookup("/repo/CMakeLists.txt")); !slices.Contains(got, "cmake") {
		t.Errorf("Lookup(CMakeLists.txt) = %v, want cmake among them", got)
	}
}

func TestLookupUnknownIsNil(t *testing.T) {
	for _, path := range []string{"x.nosuchext", "noext", ""} {
		if got := Lookup(path); got != nil {
			t.Errorf("Lookup(%q) = %v, want nil", path, names(got))
		}
	}
}

func TestLookupReturnsACopy(t *testing.T) {
	Lookup("main.go")[0].Name = "changed"
	if got := Lookup("main.go")[0].Name; got != "gopls" {
		t.Fatalf("Lookup(main.go)[0] = %q after a caller wrote to it", got)
	}
}

// eslint computes its cmd in Lua, so the catalog has no Command for it.
func TestFunctionCmdIsDynamic(t *testing.T) {
	eslint := find(t, "eslint")
	if !slices.Contains(eslint.Dynamic, "cmd") || eslint.Command != nil {
		t.Fatalf("eslint: Dynamic %v, Command %v; want cmd dynamic and no Command", eslint.Dynamic, eslint.Command)
	}
	if gopls := find(t, "gopls"); slices.Contains(gopls.Dynamic, "cmd") || !slices.Equal(gopls.Command, []string{"gopls"}) {
		t.Fatalf("gopls: Dynamic %v, Command %v; want a plain gopls command", gopls.Dynamic, gopls.Command)
	}
}

func TestACommonServerWithAFunctionCmdCarriesItsDefaultCommand(t *testing.T) {
	ts := find(t, "ts_ls")
	if !slices.Contains(ts.Dynamic, "cmd") || !slices.Equal(ts.Command, []string{"typescript-language-server", "--stdio"}) {
		t.Fatalf("ts_ls: Dynamic %v, Command %v; want cmd dynamic and its default command", ts.Dynamic, ts.Command)
	}
}

func TestGoplsInstallsWithGo(t *testing.T) {
	in := find(t, "gopls").Install
	if in == nil || in.Kind != KindGolang || in.Package != "golang.org/x/tools/gopls" || in.Version == "" {
		t.Fatalf("gopls Install = %+v, want golang.org/x/tools/gopls of KindGolang at a version", in)
	}
	if !slices.Contains(in.Bins, "gopls") {
		t.Fatalf("gopls Bins = %v, want gopls", in.Bins)
	}
}

func TestGithubRecipeHasALinuxAsset(t *testing.T) {
	for _, s := range All() {
		if s.Install == nil || s.Install.Kind != KindGithub {
			continue
		}
		for _, a := range s.Install.Assets {
			if a.Target == "linux_x64" && len(a.Files) > 0 && len(a.Bins) > 0 {
				return
			}
		}
	}
	t.Fatal("no github recipe has a linux_x64 asset with files and bins")
}

// Every template mason writes is expanded or the asset left out.
func TestAssetsCarryNoTemplates(t *testing.T) {
	for _, s := range All() {
		if s.Install == nil {
			continue
		}
		for _, a := range s.Install.Assets {
			for _, v := range append(slices.Clone(a.Files), slices.Collect(maps.Values(a.Bins))...) {
				if strings.Contains(v, "{{") {
					t.Errorf("%s %s: %q holds a template", s.Name, a.Target, v)
				}
			}
		}
	}
}
