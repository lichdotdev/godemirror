package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

var (
	flagRecord = flag.String("record", "", "headless: run this script of actions (wait, type, key, move, click), writing a PNG of every frame to -frames")
	flagFrames = flag.String("frames", "frames", "with -record, the directory the frames go to")
	flagScale  = flag.Float64("scale", 1, "headless: pixels per dp and sp, 2 for a sharp shot of the same layout")
)

// recordFrame is the time between recorded frames: 20 frames a second.
const recordFrame = 50 * time.Millisecond

// typeDelay is the time between two characters a script types.
const typeDelay = 90 * time.Millisecond

// recorder lays the demo out headless at a steady rate, in real time so a
// language server can answer, and saves each frame.
type recorder struct {
	d      *demo
	win    *headless.Window
	size   image.Point
	ops    op.Ops
	router input.Router
	n      int
}

func runRecord(d *demo) error {
	size := scaled(image.Pt(*flagWidth, *flagHeight))
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		return fmt.Errorf("headless window: %w", err)
	}
	defer win.Release()
	if err := os.MkdirAll(*flagFrames, 0o755); err != nil {
		return err
	}
	r := &recorder{d: d, win: win, size: size}
	f, err := os.Open(*flagRecord)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// A line is not trimmed: what type types may end in a space.
		if line := sc.Text(); strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
			if err := r.do(line); err != nil {
				return fmt.Errorf("%q: %w", line, err)
			}
		}
	}
	return sc.Err()
}

// do runs one line of a script: "wait 1s", "type text", "key Ctrl+Space",
// "move x,y [ctrl]", "click x,y [ctrl]", positions in dp.
func (r *recorder) do(line string) error {
	verb, arg, _ := strings.Cut(line, " ")
	switch verb {
	case "wait":
		d, err := time.ParseDuration(arg)
		if err != nil {
			return err
		}
		return r.run(d)
	case "type":
		for _, c := range arg {
			sel := r.router.EditorState().Selection.Range
			at := min(sel.Start, sel.End) + 1
			r.router.Queue(key.EditEvent{Range: sel, Text: string(c)}, key.SelectionEvent{Start: at, End: at})
			if err := r.run(typeDelay); err != nil {
				return err
			}
		}
		return nil
	case "key":
		r.router.Queue(keyPress(arg))
		return r.run(recordFrame)
	case "move", "click":
		return r.pointer(verb, arg)
	}
	return fmt.Errorf("unknown action %q", verb)
}

func (r *recorder) pointer(verb, arg string) error {
	at, ctrl, _ := strings.Cut(arg, " ")
	p, err := parseHover(at)
	if err != nil || p == nil {
		return fmt.Errorf("want x,y: %v", err)
	}
	pos := f32.Pt(p.X*float32(*flagScale), p.Y*float32(*flagScale))
	var mods key.Modifiers
	if ctrl == "ctrl" {
		mods = key.ModShortcut
	}
	ev := pointer.Event{Source: pointer.Mouse, Position: pos, Modifiers: mods, Kind: pointer.Move}
	r.router.Queue(ev)
	if verb == "click" {
		ev.Buttons = pointer.ButtonPrimary
		ev.Kind = pointer.Press
		r.router.Queue(ev)
		ev.Kind = pointer.Release
		r.router.Queue(ev)
	}
	return r.run(recordFrame)
}

// run draws and saves frames for d of real time.
func (r *recorder) run(d time.Duration) error {
	for end := time.Now().Add(d); time.Now().Before(end); {
		next := time.Now().Add(recordFrame)
		if err := r.frame(); err != nil {
			return err
		}
		time.Sleep(time.Until(next))
	}
	return nil
}

func (r *recorder) frame() error {
	r.ops.Reset()
	gtx := layout.Context{
		Ops:         &r.ops,
		Now:         time.Now(),
		Source:      r.router.Source(),
		Metric:      unit.Metric{PxPerDp: float32(*flagScale), PxPerSp: float32(*flagScale)},
		Constraints: layout.Exact(r.size),
	}
	if err := r.d.layout(gtx); err != nil {
		return err
	}
	r.router.Frame(&r.ops)
	if err := r.win.Frame(&r.ops); err != nil {
		return err
	}
	img := image.NewRGBA(image.Rectangle{Max: r.size})
	if err := r.win.Screenshot(img); err != nil {
		return err
	}
	out, err := os.Create(filepath.Join(*flagFrames, fmt.Sprintf("%04d.png", r.n)))
	if err != nil {
		return err
	}
	defer out.Close()
	r.n++
	return png.Encode(out, img)
}

// scaled is size, in dp, in pixels at -scale.
func scaled(size image.Point) image.Point {
	return image.Pt(int(float64(size.X)**flagScale), int(float64(size.Y)**flagScale))
}
