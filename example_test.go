package godemirror_test

import (
	"errors"
	"fmt"
	"image"
	"log"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"golang.org/x/image/font/gofont/gomono"

	"github.com/lichdotdev/godemirror"
)

func ExampleView() {
	view, err := godemirror.New("monospace", unit.Sp(13))
	if errors.Is(err, godemirror.ErrFontNotFound) {
		// Without an installed monospace font, hand the view the font files instead.
		view, err = godemirror.NewFromFonts(godemirror.Fonts{Regular: gomono.TTF}, unit.Sp(13))
	}
	if err != nil {
		log.Fatal(err)
	}
	view.SetContent(godemirror.Content{
		Filename: "main.go",
		Text:     "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
	})
	view.SelectLines(2, 4)
	view.ScrollToLine(2, godemirror.AlignNearest)

	// In an app, gtx comes from each app.FrameEvent of the window.
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(640, 480)),
	}
	if _, err := view.Layout(gtx); err != nil {
		log.Fatal(err)
	}

	from, to, _ := view.SelectedLines()
	fmt.Printf("%d lines; lines %d-%d selected:\n%s", view.LineCount(), from, to, view.SelectedText())
	// Output:
	// 5 lines; lines 2-4 selected:
	// func main() {
	// 	println("hi")
	// }
}

func ExampleView_SetSearch() {
	view, err := godemirror.New("monospace", unit.Sp(13))
	if errors.Is(err, godemirror.ErrFontNotFound) {
		view, err = godemirror.NewFromFonts(godemirror.Fonts{Regular: gomono.TTF}, unit.Sp(13))
	}
	if err != nil {
		log.Fatal(err)
	}
	view.SetContent(godemirror.Content{Text: "foo bar\nFoo(bar)\nbaz foo"})

	// A search box calls SetSearch as its text changes, and FindNext on Enter.
	if err := view.SetSearch(godemirror.SearchQuery{Text: "foo", WholeWord: true}); err != nil {
		log.Fatal(err)
	}
	for range 4 {
		m, _ := view.FindNext()
		fmt.Println(m.From, m.To)
	}
	// Output:
	// {0 0} {0 3}
	// {1 0} {1 3}
	// {2 4} {2 7}
	// {0 0} {0 3}
}
