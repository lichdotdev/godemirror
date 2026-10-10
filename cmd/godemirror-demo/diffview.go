package main

import (
	"errors"
	"log"
	"os"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
)

// diffViewDemo shows a diff through godemirror.DiffView: -old and -file
// diffed in the library, or with -diffview the first file of -diff, its gaps
// filled from -new.
type diffViewDemo struct {
	view     *godemirror.DiffView
	newLines []string
}

func newDiffViewDemo() (*diffViewDemo, error) {
	d := &diffViewDemo{}
	opts := godemirror.DiffOptions{
		Collapse: &godemirror.Collapse{Margin: 3, MinSize: 4},
		Expand:   d.expand,
		Revert:   func(c godemirror.Chunk) { log.Printf("revert %+v", c) },

		HighlightChanges: *flagChanges,
	}
	view, err := godemirror.NewDiffView(*flagFont, unit.Sp(*flagSize), opts)
	if err != nil {
		return nil, err
	}
	d.view = view
	view.Split, view.Wrap = *flagSplit, *flagWrap
	if *flagLight {
		view.SetTheme(godemirror.LightTheme)
	}
	if *flagOld != "" {
		return d, d.loadDocs()
	}
	return d, d.loadPatch()
}

func (d *diffViewDemo) loadDocs() error {
	original, err := os.ReadFile(*flagOld)
	if err != nil {
		return err
	}
	modified, err := os.ReadFile(*flagFile)
	if err != nil {
		return err
	}
	d.view.SetDocs(*flagFile, string(original), string(modified))
	return nil
}

func (d *diffViewDemo) loadPatch() error {
	patch, err := os.ReadFile(*flagDiff)
	if err != nil {
		return err
	}
	files, err := godemirror.ParsePatch(string(patch))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("-diff: no file in the patch")
	}
	if *flagNew != "" {
		text, err := os.ReadFile(*flagNew)
		if err != nil {
			return err
		}
		d.newLines = strings.Split(string(text), "\n")
	}
	d.view.SetPatch(files[0])
	return nil
}

// expand answers from -new, at most fillPart lines at a time, the way a
// backend that caps its answers would.
func (d *diffViewDemo) expand(g godemirror.Gap) {
	const fillPart = 500
	for g.From <= g.To && g.From <= len(d.newLines) {
		part := d.newLines[g.From-1 : min(g.To, g.From-1+fillPart, len(d.newLines))]
		d.view.FillGap(g, part)
		g.From += len(part)
	}
}

func (d *diffViewDemo) layout(gtx layout.Context) error {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: keyWrap, Required: key.ModAlt},
			key.Filter{Name: keySplit, Required: key.ModAlt},
		)
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			switch e.Name {
			case keyWrap:
				d.view.Wrap = !d.view.Wrap
			case keySplit:
				d.view.Split = !d.view.Split
			}
		}
	}
	_, err := d.view.Layout(gtx)
	return err
}
