package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"

	"github.com/lichdotdev/godemirror"
)

// complete is the demo's Complete: after a dot, the fields and methods of
// what is before it, as go/types reads the file; anything else, or a dot it
// cannot type (a package the browser does not have), the file's words.
func complete(ctx context.Context, req godemirror.CompletionRequest) godemirror.CompletionResult {
	if items := completeSelector(req); len(items) > 0 {
		return godemirror.CompletionResult{Items: items}
	}
	return completeWords(ctx, req)
}

// selectorHole stands for the name being typed after a dot when none is yet,
// so the file parses with a selector there.
const selectorHole = "_"

// completeSelector is the fields and methods of x in the x.name being typed
// at req.Pos, nil when the cursor is not after a dot or x has no type here.
// The imports are not type-checked: a browser has no packages to read.
func completeSelector(req godemirror.CompletionRequest) []godemirror.CompletionItem {
	doc := req.State.Doc()
	if req.From == 0 || doc[req.From-1] != '.' {
		return nil
	}
	src := doc[:req.Pos]
	if req.From == req.Pos {
		src += selectorHole
	}
	src += doc[req.Pos:]
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "server.go", src, parser.SkipObjectResolution)
	if file == nil {
		return nil
	}
	sel := selectorAt(file, fset.File(file.Pos()).Pos(req.From))
	if sel == nil {
		return nil
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Importer: emptyImporter{}, Error: func(error) {}}
	pkg, _ := conf.Check(file.Name.Name, fset, []*ast.File{file}, info)
	tv, ok := info.Types[sel.X]
	if !ok || tv.Type == nil || !tv.IsValue() {
		return nil
	}
	return members(tv.Type, pkg, declaredTypes(fset, file, src), req)
}

// declaredTypes are, by the position of a field's or a method's name in
// file, its type as the source writes it: go/types prints a type from a
// package the browser does not have as invalid.
func declaredTypes(fset *token.FileSet, file *ast.File, src string) map[token.Pos]string {
	text := func(n ast.Node) string {
		return src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset]
	}
	out := map[token.Pos]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			for _, name := range n.Names {
				out[name.Pos()] = text(n.Type)
			}
		case *ast.FuncDecl:
			sig := n.Name.Name + text(n.Type.Params)
			if n.Type.Results != nil {
				sig += " " + text(n.Type.Results)
			}
			out[n.Name.Pos()] = sig
		}
		return true
	})
	return out
}

// selectorAt is the selector expression whose name starts at pos.
func selectorAt(file *ast.File, pos token.Pos) *ast.SelectorExpr {
	var found *ast.SelectorExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Pos() == pos {
			found = s
		}
		return found == nil
	})
	return found
}

// members are t's fields and methods as completions, each with its type as
// written where declared, or as pkg would print it.
func members(t types.Type, pkg *types.Package, written map[token.Pos]string, req godemirror.CompletionRequest) []godemirror.CompletionItem {
	qual := types.RelativeTo(pkg)
	var items []godemirror.CompletionItem
	add := func(obj types.Object, kind godemirror.CompletionKind, detail string) {
		if w, ok := written[obj.Pos()]; ok {
			detail = w
		}
		items = append(items, godemirror.CompletionItem{Label: obj.Name(), Kind: kind, Detail: detail, From: req.From, To: req.Pos})
	}
	if s, ok := deref(t).Underlying().(*types.Struct); ok {
		for f := range s.Fields() {
			add(f, godemirror.CompletionProperty, types.TypeString(f.Type(), qual))
		}
	}
	mset := types.NewMethodSet(t)
	if _, isPtr := t.(*types.Pointer); !isPtr && !types.IsInterface(t) {
		mset = types.NewMethodSet(types.NewPointer(t))
	}
	for m := range mset.Methods() {
		sig := strings.TrimPrefix(types.TypeString(m.Type(), qual), "func")
		add(m.Obj(), godemirror.CompletionMethod, m.Obj().Name()+sig)
	}
	return items
}

func deref(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// emptyImporter gives every import as an empty package: the browser has
// none to read, and a selector on one finds nothing.
type emptyImporter struct{}

func (emptyImporter) Import(path string) (*types.Package, error) {
	name := path[strings.LastIndex(path, "/")+1:]
	pkg := types.NewPackage(path, name)
	pkg.MarkComplete()
	return pkg, nil
}
