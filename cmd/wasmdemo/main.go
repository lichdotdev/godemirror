// Command wasmdemo is godemirror in a browser: an Editor on a Go file, the
// same file's diff against an earlier version, and the themes, with no
// language server (a browser runs no processes).
//
//	GOOS=js GOARCH=wasm go build -o site/main.wasm ./cmd/wasmdemo
package main

import (
	_ "embed"
	"image"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/godemirror"
)

var (
	//go:embed testdata/server.go.txt
	modified string
	//go:embed testdata/server_old.go.txt
	original string
)

// codeSize is the size the demo draws code at.
const codeSize = unit.Sp(14)

// themes are the themes the Themes button cycles through.
var themes = []struct {
	name  string
	theme godemirror.Theme
}{
	{"Dark", godemirror.DarkTheme}, {"Light", godemirror.LightTheme}, {"One Dark", godemirror.OneDark},
	{"GitHub Light", godemirror.GitHubLight}, {"Dracula", godemirror.Dracula}, {"Nord", godemirror.Nord},
}

// control are the changes the page asks for, run on the UI goroutine at
// the next frame.
var control = make(chan func(*demo), 64)

type demo struct {
	editor                               *godemirror.Editor
	diff                                 *godemirror.DiffView
	th                                   *material.Theme
	editBtn, diffBtn, themeBtn, splitBtn widget.Clickable
	showDiff                             bool
	theme                                int
	focused                              bool
	// embedded hides the demo's own buttons when the page draws tabs.
	embedded bool
}

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("godemirror"))
		// send runs f at the next frame, in order; a call from the page goes
		// through it on a goroutine of its own, since the page calls on the
		// browser's event loop, which the window needs free to draw.
		send := func(f func(*demo)) {
			control <- f
			w.Invalidate()
		}
		listen(send)
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

// editorConfig is the file the editor opens with, and goes back to before
// the scripted edit plays.
func editorConfig() godemirror.EditorStateConfig {
	return godemirror.EditorStateConfig{Filename: "server.go", Doc: modified, AllowMultipleSelections: true, IndentUnit: "\t", BracketIndent: true}
}

func newDemo() (*demo, error) {
	f, family := fonts()
	var (
		editor *godemirror.Editor
		diff   *godemirror.DiffView
		err    error
	)
	if family != "" {
		editor, err = godemirror.NewEditor(family, codeSize)
	} else {
		editor, err = godemirror.NewEditorFromFonts(f, codeSize)
	}
	if err != nil {
		return nil, err
	}
	opts := godemirror.DiffOptions{Collapse: &godemirror.Collapse{Margin: 3, MinSize: 4}, HighlightChanges: true}
	if family != "" {
		diff, err = godemirror.NewDiffView(family, codeSize, opts)
	} else {
		diff, err = godemirror.NewDiffViewFromFonts(f, codeSize, opts)
	}
	if err != nil {
		return nil, err
	}
	if err := editor.SetState(editorConfig()); err != nil {
		return nil, err
	}
	editor.RectangularSelection = true
	editor.Complete, editor.CompletionTriggers = complete, []string{"."}
	editor.IndentWithTab, editor.CloseBrackets = true, true
	diff.Split = true
	diff.SetDocs("server.go", original, modified)
	th := material.NewTheme()
	// A browser has no system fonts for Gio to find: the buttons are set in
	// Gio's own Go fonts.
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	return &demo{editor: editor, diff: diff, th: th}, nil
}

func run(w *app.Window) error {
	d, err := newDemo()
	if err != nil {
		return err
	}
	var ops op.Ops
	first := true
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if err := d.layout(gtx); err != nil {
				return err
			}
			e.Frame(gtx.Ops)
			if first {
				markFirstFrame()
				first = false
			}
		}
	}
}

func (d *demo) layout(gtx layout.Context) error {
	for drained := false; !drained; {
		select {
		case f := <-control:
			f(d)
		default:
			drained = true
		}
	}
	d.handleButtons(gtx)
	t := themes[d.theme].theme
	d.editor.Theme = t
	d.diff.SetTheme(t)
	paint.FillShape(gtx.Ops, t.GutterBackground, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
	var err error
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if d.embedded {
				return layout.Dimensions{}
			}
			return d.bar(gtx, t)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			var dims layout.Dimensions
			if d.showDiff {
				dims, err = d.diff.Layout(gtx)
				return dims
			}
			if !d.focused {
				d.editor.Focus(gtx)
				d.focused = true
			}
			dims, err = d.editor.Layout(gtx)
			return dims
		}),
	)
	return err
}

func (d *demo) handleButtons(gtx layout.Context) {
	if d.editBtn.Clicked(gtx) {
		d.showDiff, d.focused = false, false
	}
	if d.diffBtn.Clicked(gtx) {
		d.showDiff = true
	}
	if d.splitBtn.Clicked(gtx) {
		d.diff.Split = !d.diff.Split
	}
	if d.themeBtn.Clicked(gtx) {
		d.theme = (d.theme + 1) % len(themes)
	}
}

// bar is the row of buttons over the code, drawn in the code's theme.
func (d *demo) bar(gtx layout.Context, t godemirror.Theme) layout.Dimensions {
	d.th.Palette.Bg, d.th.Palette.Fg = t.Background, t.Foreground
	d.th.Palette.ContrastBg, d.th.Palette.ContrastFg = t.Popup.Selected, t.Popup.SelectedForeground
	button := func(c *widget.Clickable, label string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(4)).Layout(gtx, material.Button(d.th, c, label).Layout)
		})
	}
	split := "Unified"
	if !d.diff.Split {
		split = "Side by side"
	}
	children := []layout.FlexChild{button(&d.editBtn, "Editor"), button(&d.diffBtn, "Diff")}
	if d.showDiff {
		children = append(children, button(&d.splitBtn, split))
	}
	children = append(children, button(&d.themeBtn, "Theme: "+themes[d.theme].name))
	return layout.Flex{}.Layout(gtx, children...)
}
