package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/godemirror/lsp"
)

var (
	flagLSP  = flag.String("lsp", "", "with -edit, a language server command to attach (gopls), run in -file's directory")
	flagLang = flag.String("lang", "go", "with -lsp, the document's LSP language id")
	flagAt   = flag.String("at", "", "with -edit, a line:col (from 1) to put the cursor at")
	flagType = flag.String("type", "", "headless, with -edit: text typed at the cursor, a character a frame, before the shot")
	flagCtrl = flag.Bool("ctrl", false, "headless: hold Ctrl with the pointer at -hover")
	flagWait = flag.Duration("wait", 0, "headless: keep drawing frames this long before the shot, for a server's answers")
)

// lspStart bounds starting the server and its handshake.
const lspStart = 30 * time.Second

// attachLSP starts -lsp in -file's directory and binds the editor to it;
// invalidate asks the window for a frame.
func (d *demo) attachLSP(invalidate func()) error {
	if *flagLSP == "" || d.editor == nil {
		return nil
	}
	path, err := filepath.Abs(*flagFile)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), lspStart)
	defer cancel()
	c, err := lsp.Start(ctx, lsp.Config{Command: strings.Fields(*flagLSP), RootDir: filepath.Dir(path), Stderr: os.Stderr})
	if err != nil {
		return err
	}
	d.client = c
	_, err = lsp.Attach(c, d.editor, lsp.FileURI(path), lsp.Options{
		LanguageID: *flagLang, Invalidate: invalidate,
		OnError: func(err error) { log.Print(err) },
	})
	d.editor.OnDefinition = func(l godemirror.Location) {
		log.Printf("definition in %s at %d:%d", l.URI, l.Start.Line+1, l.Start.Column+1)
	}
	return err
}

// placeCursor puts the editor's cursor at -at.
func (d *demo) placeCursor() error {
	if *flagAt == "" || d.editor == nil {
		return nil
	}
	var line, col int
	if _, err := fmt.Sscanf(*flagAt, "%d:%d", &line, &col); err != nil {
		return fmt.Errorf("-at %q: want line:col", *flagAt)
	}
	s := d.editor.State()
	l := s.Line(min(max(line, 1), s.Lines()))
	pos := l.From + min(max(col-1, 0), len(l.Text))
	return d.editor.Dispatch(godemirror.TransactionSpec{Selection: godemirror.SingleSelection(pos, pos), ScrollIntoView: true})
}

// waiting is how far the headless demo is in typing -type, a character
// a frame, and until when it waits for -wait.
type waiting struct {
	typed int
	until time.Time
}

// typeNext queues the next character of -type as Gio's window types it,
// and reports whether there was one.
func (w *waiting) typeNext(router *input.Router) bool {
	rest := []rune(*flagType)[w.typed:]
	if len(rest) == 0 {
		return false
	}
	w.typed++
	sel := router.EditorState().Selection.Range
	at := min(sel.Start, sel.End) + 1
	router.Queue(key.EditEvent{Range: sel, Text: string(rest[0])}, key.SelectionEvent{Start: at, End: at})
	return true
}

// lastFrameWaits reports whether the frame that would be the last is not
// yet: -type is typed first, a character a frame, then -wait passes, the
// pointer resting at hover.
func (w *waiting) lastFrameWaits(router *input.Router, hover *f32.Point) bool {
	if w.typeNext(router) {
		return true
	}
	if *flagWait == 0 {
		return false
	}
	if w.until.IsZero() {
		w.until = time.Now().Add(*flagWait)
		if hover != nil {
			router.Queue(hoverEvent(*hover))
		}
	}
	return time.Now().Before(w.until)
}

// hoverEvent moves the pointer to at, Ctrl held with -ctrl.
func hoverEvent(at f32.Point) pointer.Event {
	ev := pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at}
	if *flagCtrl {
		ev.Modifiers = key.ModShortcut
	}
	return ev
}
