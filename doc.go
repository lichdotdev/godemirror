// Package godemirror is a code editor library for Gio, native to Go: an
// Editor with multiple cursors, undo history, snippets and the hooks a
// language server fills (diagnostics, completion, hover, signature help, go
// to definition, references, rename, code actions); a read-only View, and
// Split for two documents side by side; and a DiffView, unified or side by
// side, with an optional revert on each change. Code is drawn as one image per visible
// row and highlighted by Lezer grammars running natively, off the UI
// goroutine.
//
// An Editor on a file, laid out every frame:
//
//	editor, err := godemirror.NewEditor("monospace", unit.Sp(13))
//	if err != nil {
//		return err
//	}
//	err = editor.SetState(godemirror.EditorStateConfig{Filename: "main.go", Doc: source})
//	if err != nil {
//		return err
//	}
//	editor.OnUpdate = func(u godemirror.Update) {
//		if u.DocChanged() {
//			save(u.State.Doc())
//		}
//	}
//
//	// in the frame loop, for each app.FrameEvent
//	if _, err := editor.Layout(gtx); err != nil {
//		return err
//	}
//
// Package lsp attaches a language server to an Editor, and lsp/servers tells
// which server a file wants and how to install it.
//
// New and NewSplit build the read-only views, resolving a font family among
// the installed fonts; NewFromFonts takes the font files themselves. Give a
// view a Content and call Layout every frame. NewDiffView builds a diff of
// two documents, or of a file of a git patch.
//
// A View, a Split, a DiffView or an Editor belongs to the goroutine that lays
// it out: call its methods and set its fields there, between frames. Only
// highlighting, and an Editor's hooks, run on goroutines of their own, and
// their results are taken in by a later Layout; Editor.Post runs a function
// on the layout goroutine from any other.
//
// The deliberate limits, and the traps they set for a caller, are in
// docs/ceilings.md.
package godemirror
