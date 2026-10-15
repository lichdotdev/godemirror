package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"slices"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/lichdotdev/godemirror"
)

// Keys, held with Alt, that drive the demo while it runs: Alt keeps them
// apart from what is typed into a comment.
const (
	keyWrap    = "W"
	keySplit   = "S"
	keyComment = "N"
)

// fillPart is how many lines the demo hands FillGap at a time, the way a
// backend that caps its answers would.
const fillPart = 500

// demo shows -file in a View, or a diff in a DiffView: -old against -file,
// diffed by the library, or the first file of the -diff patch.
type demo struct {
	file *godemirror.View
	diff *godemirror.DiffView
	// theme is the views' theme, which the comments take their colors from.
	theme godemirror.Theme
	th    *material.Theme

	// original and modified are the two documents of -old, which a revert
	// edits; newLines is -new, which fills the gaps of a patch.
	filename           string
	original, modified string
	newLines           []string

	comments []*comment
}

var portedThemes = map[string]godemirror.Theme{
	"onedark":     godemirror.OneDark,
	"dracula":     godemirror.Dracula,
	"githublight": godemirror.GitHubLight,
	"githubdark":  godemirror.GitHubDark,
	"nord":        godemirror.Nord,
}

func newDemo() (*demo, error) {
	d := &demo{theme: godemirror.DarkTheme, th: material.NewTheme()}
	if *flagLight {
		d.theme = godemirror.LightTheme
	}
	if *flagTheme != "" {
		t, ok := portedThemes[*flagTheme]
		if !ok {
			return nil, fmt.Errorf("-theme %q: not a ported theme", *flagTheme)
		}
		d.theme = t
	}
	if *flagOld == "" && *flagDiff == "" {
		return d, d.loadFile()
	}
	opts := godemirror.DiffOptions{
		Collapse:         &godemirror.Collapse{Margin: 3, MinSize: 4},
		Expand:           d.expand,
		Revert:           d.revert,
		HighlightChanges: *flagChanges,
	}
	var err error
	if d.diff, err = godemirror.NewDiffView(*flagFont, unit.Sp(*flagSize), opts); err != nil {
		return nil, err
	}
	d.diff.SetTheme(d.theme)
	d.diff.Split, d.diff.Wrap = *flagSplit, *flagWrap
	if *flagOld != "" {
		return d, d.loadDocs()
	}
	return d, d.loadPatch()
}

func (d *demo) loadFile() error {
	text, err := os.ReadFile(*flagFile)
	if err != nil {
		return err
	}
	if d.file, err = godemirror.New(*flagFont, unit.Sp(*flagSize)); err != nil {
		return err
	}
	d.file.Theme, d.file.Wrap = d.theme, *flagWrap
	d.file.SetContent(godemirror.Content{Filename: *flagFile, Text: string(text)})
	return nil
}

func (d *demo) loadDocs() error {
	original, err := os.ReadFile(*flagOld)
	if err != nil {
		return err
	}
	modified, err := os.ReadFile(*flagFile)
	if err != nil {
		return err
	}
	d.filename, d.original, d.modified = *flagFile, string(original), string(modified)
	d.diff.SetDocs(d.filename, d.original, d.modified)
	return nil
}

func (d *demo) loadPatch() error {
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
	d.diff.SetPatch(files[0])
	return nil
}

// search highlights the matches of text and selects the first one, as a
// caller's search box would on Enter. Empty text searches nothing.
func (d *demo) search(text string) error {
	if text == "" {
		return nil
	}
	q := godemirror.SearchQuery{Text: text}
	if d.diff == nil {
		if err := d.file.SetSearch(q); err != nil {
			return err
		}
		d.file.FindNext()
		return nil
	}
	if err := d.diff.SetSearch(q); err != nil {
		return err
	}
	d.diff.FindNext()
	return nil
}

// expand answers a gap of the patch from -new, fillPart lines at a time.
func (d *demo) expand(g godemirror.Gap) {
	for g.From <= g.To && g.From <= len(d.newLines) {
		part := d.newLines[g.From-1 : min(g.To, g.From-1+fillPart, len(d.newLines))]
		d.diff.FillGap(g, part)
		g.From += len(part)
	}
}

// revert puts a chunk of -old back into the modified document and shows the
// diff again, in place. A patch has no document to edit, so its reverts are
// only logged.
func (d *demo) revert(c godemirror.Chunk) {
	if *flagOld == "" {
		log.Printf("revert %+v", c)
		return
	}
	var old []string
	for _, l := range c.Lines {
		if l.Side == godemirror.OldSide {
			old = append(old, l.Text)
		}
	}
	lines := strings.Split(d.modified, "\n")
	at := min(c.NewStart-1, len(lines))
	lines = slices.Replace(lines, at, min(at+c.NewCount, len(lines)), old...)
	d.modified = strings.Join(lines, "\n")
	d.diff.SetDocs(d.filename, d.original, d.modified)
}

// layout registers the demo's key handler beneath the view: an area handler
// on top would catch the pointer's scrolling before the view does.
func (d *demo) layout(gtx layout.Context) error {
	d.handleKeys(gtx)
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, d)
	area.Pop()
	if d.diff == nil {
		_, err := d.file.Layout(gtx)
		return err
	}
	_, err := d.diff.Layout(gtx)
	return err
}

// handleKeys takes its keys whatever has focus, so a view keeps the focus its
// selection's shortcuts need.
func (d *demo) handleKeys(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: keyWrap, Required: key.ModAlt},
			key.Filter{Name: keySplit, Required: key.ModAlt},
			key.Filter{Name: keyComment, Required: key.ModAlt},
		)
		if !ok {
			return
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			d.onKey(e.Name)
		}
	}
}

func (d *demo) onKey(name key.Name) {
	if d.diff == nil {
		if name == keyWrap {
			d.file.Wrap = !d.file.Wrap
		}
		return
	}
	switch name {
	case keyWrap:
		d.diff.Wrap = !d.diff.Wrap
	case keySplit:
		d.diff.Split = !d.diff.Split
	case keyComment:
		d.openComment()
	}
}
