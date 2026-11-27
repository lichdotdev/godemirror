// Command lsp opens a file in a godemirror Editor with its language server
// attached: diagnostics, completions, hover, signature help, go to
// definition, rename and code actions. The server comes from the
// lsp/servers catalog, and must be installed.
//
//	go run ./cmd/examples/lsp path/to/main.go
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/godemirror/lsp"
	"github.com/lichdotdev/godemirror/lsp/servers"
)

// startTimeout bounds starting the server and its handshake.
const startTimeout = 30 * time.Second

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: lsp path/to/file")
	}
	go func() {
		if err := run(os.Args[1]); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(file string) error {
	path, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	editor, err := godemirror.NewEditor("monospace", unit.Sp(13))
	if err != nil {
		return err
	}
	if err := editor.SetState(godemirror.EditorStateConfig{Filename: path, Doc: string(text)}); err != nil {
		return err
	}
	w := new(app.Window)
	w.Option(app.Title(filepath.Base(path)))
	stop, err := attach(editor, path, w.Invalidate)
	if err != nil {
		return err
	}
	defer stop()
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if !editor.HasFocus(gtx) {
				editor.Focus(gtx)
			}
			if _, err := editor.Layout(gtx); err != nil {
				return err
			}
			e.Frame(gtx.Ops)
		}
	}
}

// attach starts the first server the catalog has for path, in the project
// its root markers find, and binds the editor to it; with no server for the
// file, the editor edits alone.
func attach(editor *godemirror.Editor, path string, invalidate func()) (stop func(), err error) {
	candidates := servers.Lookup(path)
	if len(candidates) == 0 || candidates[0].Command == nil {
		return func() {}, nil
	}
	srv := candidates[0]
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	client, err := lsp.Start(ctx, lsp.Config{
		Command: srv.Command, RootDir: projectRoot(path, srv.RootMarkers),
		InitializationOptions: srv.InitOptions, Settings: srv.Settings,
	})
	if err != nil {
		return nil, err
	}
	doc, err := lsp.Attach(client, editor, lsp.FileURI(path), lsp.Options{
		LanguageID: servers.LanguageID(servers.FileType(path)),
		Invalidate: invalidate,
		OnError:    func(err error) { log.Print(err) },
	})
	if err != nil {
		logErr(client.Close())
		return nil, err
	}
	return func() { logErr(doc.Close()); logErr(client.Close()) }, nil
}

func logErr(err error) {
	if err != nil {
		log.Print(err)
	}
}

// projectRoot is the nearest directory above path holding one of markers,
// the groups tried in order; path's own directory when none does.
func projectRoot(path string, markers [][]string) string {
	for _, group := range markers {
		for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
			for _, m := range group {
				if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
					return dir
				}
			}
			if parent := filepath.Dir(dir); parent == dir {
				break
			}
		}
	}
	return filepath.Dir(path)
}
