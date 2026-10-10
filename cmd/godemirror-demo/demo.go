package main

import (
	"image"
	"os"
	"slices"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
)

// Keys, held with Alt, that drive the demo while it runs: Alt keeps them
// apart from what is typed into a comment.
const (
	keyWrap    = "W"
	keySplit   = "S"
	keyComment = "N"
)

// demo holds both layouts of what is shown, so switching between them keeps
// each one's scroll position, and the diff model the buttons act on.
type demo struct {
	unified   *godemirror.View
	split     *godemirror.Split
	showSplit bool
	model     *diffModel
	buttons   *buttons
	threads   []*thread
	// sides is the layout last shown, which a selection's lines index into.
	sides struct{ unified, left, right side }
}

func newDemo() (*demo, error) {
	d := &demo{showSplit: *flagSplit}
	var err error
	if d.unified, err = godemirror.New(*flagFont, unit.Sp(*flagSize)); err != nil {
		return nil, err
	}
	if d.split, err = godemirror.NewSplit(*flagFont, unit.Sp(*flagSize)); err != nil {
		return nil, err
	}
	if d.buttons, err = newButtons(); err != nil {
		return nil, err
	}
	d.setWrap(*flagWrap)
	if *flagLight {
		d.unified.Theme = godemirror.LightTheme
		d.split.SetTheme(godemirror.LightTheme)
	}
	return d, d.load()
}

func (d *demo) setWrap(wrap bool) {
	d.unified.Wrap = wrap
	d.split.Wrap = wrap
}

func (d *demo) load() error {
	if *flagDiff == "" {
		text, err := os.ReadFile(*flagFile)
		if err != nil {
			return err
		}
		d.unified.SetContent(godemirror.Content{Filename: *flagFile, Text: string(text)})
		return nil
	}
	patch, err := os.ReadFile(*flagDiff)
	if err != nil {
		return err
	}
	file, err := parseFileDiff(string(patch))
	if err != nil {
		return err
	}
	d.model = &diffModel{file: file, expanded: map[int]bool{}, reverted: map[int]bool{}}
	if *flagNew != "" {
		text, err := os.ReadFile(*flagNew)
		if err != nil {
			return err
		}
		d.model.newText = strings.Split(string(text), "\n")
	}
	u, l, r := d.contents()
	d.unified.SetContent(u)
	d.split.Left.SetContent(l)
	d.split.Right.SetContent(r)
	return nil
}

// rebuild shows the model again after a button changed it, in place.
func (d *demo) rebuild() {
	u, l, r := d.contents()
	d.unified.UpdateContent(u)
	d.split.Left.UpdateContent(l)
	d.split.Right.UpdateContent(r)
}

func (d *demo) contents() (unifiedContent, left, right godemirror.Content) {
	lines := d.model.lines()
	d.sides.unified = unified(lines)
	d.sides.left, d.sides.right = split(lines)
	none := func(int) bool { return false }
	onRight := func(anchor int) bool { return slices.Contains(d.sides.right.ids, anchor) }
	return d.content(d.sides.unified, none), d.content(d.sides.left, onRight), d.content(d.sides.right, none)
}

// content shows s; skip leaves out the threads another side already shows.
func (d *demo) content(s side, skip func(anchor int) bool) godemirror.Content {
	c := godemirror.Content{
		Filename:    d.model.file.path,
		Text:        s.text,
		Labels:      s.labels,
		Decorations: s.decorations,
		Blocks:      d.threadBlocks(s, skip),
	}
	for line, g := range s.gaps {
		c.Widgets = append(c.Widgets, godemirror.LineWidget{
			Line:      line,
			Placement: godemirror.AfterText,
			Widget: d.buttons.expandButton(g, func() {
				d.model.expanded[g.key] = true
				d.rebuild()
			}),
		})
	}
	for block, line := range s.reverts {
		c.Widgets = append(c.Widgets, godemirror.LineWidget{
			Line:         line,
			Placement:    godemirror.RightEdge,
			WhileHovered: true,
			Widget: d.buttons.revertButton(block, func() {
				d.model.reverted[block] = true
				d.rebuild()
			}),
		})
	}
	return c
}

// layout registers the demo's key handler beneath the views: an area handler
// on top would catch the pointer's scrolling before the views do.
func (d *demo) layout(gtx layout.Context) error {
	d.handleKeys(gtx)
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, d)
	area.Pop()
	if d.showSplit && d.model != nil {
		_, err := d.split.Layout(gtx)
		return err
	}
	_, err := d.unified.Layout(gtx)
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
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case keyWrap:
			d.setWrap(!d.unified.Wrap)
		case keySplit:
			d.showSplit = !d.showSplit
		case keyComment:
			if d.model != nil {
				d.openComposer()
			}
		}
	}
}
