// Demo of godemirror: a file in a View, or a diff in a DiffView, in a window
// or, with -bench or -shot, headless. Headless, it scrolls frame by frame
// through Gio's own input router and reports frame times or writes a PNG.
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"slices"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror/internal/shot"
)

var (
	flagFile    = flag.String("file", "view.go", "file to show; with -old, the modified version")
	flagOld     = flag.String("old", "", "show the diff of this file against -file")
	flagDiff    = flag.String("diff", "", "show the first file of this unified diff (git diff)")
	flagNew     = flag.String("new", "", "with -diff, the new file's text, which fills the lines the diff left out")
	flagChanges = flag.Bool("changes", false, "in a diff, tint the words that changed within a line")
	flagSplit   = flag.Bool("split", false, "start a diff side by side")
	flagWrap    = flag.Bool("wrap", false, "wrap long lines instead of scrolling sideways")
	flagSearch  = flag.String("search", "", "highlight the matches of this text, and select the first one")
	flagLight   = flag.Bool("light", false, "use godemirror.LightTheme")
	flagFont    = flag.String("font", "JetBrainsMono Nerd Font", "font family, resolved to files by fc-match")
	flagSize    = flag.Float64("size", 13, "font size in sp")
	flagWidth   = flag.Int("width", 1200, "window width in px")
	flagHeight  = flag.Int("height", 800, "window height in px")
	flagShot    = flag.String("shot", "", "headless: write a PNG of the last frame here")
	flagBench   = flag.Int("bench", 0, "headless: frames to draw, scrolling between each")
	flagStep    = flag.Float64("step", 60, "headless: pixels scrolled down per frame")
	flagSide    = flag.Float64("sidestep", 15, "headless: pixels scrolled right per frame")
	flagHover   = flag.String("hover", "", "headless: an x,y in px the pointer rests at before the last frame")
	flagSettle  = flag.Duration("settle", time.Second, "headless: wait this long before the first frame, for highlighting to finish")
)

func main() {
	flag.Parse()
	d, err := newDemo()
	if err != nil {
		log.Fatal(err)
	}
	if err := d.search(*flagSearch); err != nil {
		log.Fatal(err)
	}
	if *flagShot != "" || *flagBench > 0 {
		if err := runHeadless(d); err != nil {
			log.Fatal(err)
		}
		return
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("godemirror: Alt+W wrap, Alt+S side by side, Alt+N comment on the selection"), app.Size(unit.Dp(*flagWidth), unit.Dp(*flagHeight)))
		if err := runWindow(w, d); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func runWindow(w *app.Window, d *demo) error {
	var ops op.Ops
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
		}
	}
}

func runHeadless(d *demo) error {
	size := image.Pt(*flagWidth, *flagHeight)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		return fmt.Errorf("headless window: %w", err)
	}
	defer win.Release()
	var (
		ops    op.Ops
		router input.Router
		times  []time.Duration
		// layouts is the CPU side of each frame, before the GPU renders it.
		layouts []time.Duration
	)
	time.Sleep(*flagSettle)
	hover, err := parseHover(*flagHover)
	if err != nil {
		return err
	}
	frames := max(*flagBench, 1) + 1
	for i := range frames {
		if i == frames-1 && hover != nil {
			router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: *hover})
		} else if i > 0 && *flagBench > 0 {
			router.Queue(pointer.Event{
				Kind:     pointer.Scroll,
				Source:   pointer.Mouse,
				Position: f32.Pt(float32(size.X)/2, float32(size.Y)/2),
				Scroll:   f32.Pt(float32(*flagSide), float32(*flagStep)),
			})
		}
		start := time.Now()
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Now:         start,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
		}
		if err := d.layout(gtx); err != nil {
			return err
		}
		router.Frame(&ops)
		layouts = append(layouts, time.Since(start))
		if err := win.Frame(&ops); err != nil {
			return err
		}
		times = append(times, time.Since(start))
	}
	if *flagBench > 0 {
		report("frame", times[1:len(times)-1])
		report("layout", layouts[1:len(layouts)-1])
	}
	if *flagShot != "" {
		return shot.Write(*flagShot, size, &ops)
	}
	return nil
}

func report(what string, times []time.Duration) {
	if len(times) == 0 {
		return
	}
	var sum time.Duration
	for _, t := range times {
		sum += t
	}
	worst := slices.Index(times, slices.Max(times))
	sorted := slices.Sorted(slices.Values(times))
	fmt.Printf("%-6s frames %d  avg %.2fms  p99 %.2fms  max %.2fms at frame %d\n", what, len(times),
		ms(sum/time.Duration(len(times))), ms(sorted[len(sorted)*99/100]), ms(sorted[len(sorted)-1]), worst+1)
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func parseHover(s string) (*f32.Point, error) {
	if s == "" {
		return nil, nil
	}
	var x, y float32
	if _, err := fmt.Sscanf(s, "%f,%f", &x, &y); err != nil {
		return nil, fmt.Errorf("-hover %q: want x,y", s)
	}
	return &f32.Point{X: x, Y: y}, nil
}
