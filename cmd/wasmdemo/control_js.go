//go:build js

package main

import (
	"sync/atomic"
	"syscall/js"
)

// listen lets the page drive the demo: godemirrorShow("editor" or "diff"),
// godemirrorSplit(bool), godemirrorTheme(index), godemirrorWrap(bool),
// godemirrorPlay() for the scripted edit, and godemirrorEmbed() to hide the
// demo's own buttons. The page hears they are there through godemirrorReady,
// after the first frame.
func listen(send func(func(*demo))) {
	do := func(f func(*demo)) { go send(f) }
	g := js.Global()
	g.Set("godemirrorShow", js.FuncOf(func(_ js.Value, args []js.Value) any {
		diff := len(args) > 0 && args[0].String() == "diff"
		do(func(d *demo) { d.showDiff, d.focused = diff, false })
		return nil
	}))
	g.Set("godemirrorSplit", js.FuncOf(func(_ js.Value, args []js.Value) any {
		split := len(args) > 0 && args[0].Truthy()
		do(func(d *demo) { d.diff.Split = split })
		return nil
	}))
	g.Set("godemirrorTheme", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			i := args[0].Int()
			do(func(d *demo) { d.theme = (i%len(themes) + len(themes)) % len(themes) })
		}
		return nil
	}))
	var playing atomic.Bool
	g.Set("godemirrorPlay", js.FuncOf(func(js.Value, []js.Value) any {
		// A play asked while one runs is dropped: the two would type into
		// each other.
		if playing.CompareAndSwap(false, true) {
			go func() {
				play(send)
				playing.Store(false)
			}()
		}
		return nil
	}))
	g.Set("godemirrorWrap", js.FuncOf(func(_ js.Value, args []js.Value) any {
		wrap := len(args) > 0 && args[0].Truthy()
		do(func(d *demo) { d.editor.Wrap = wrap })
		return nil
	}))
	g.Set("godemirrorEmbed", js.FuncOf(func(js.Value, []js.Value) any {
		do(func(d *demo) { d.embedded = true })
		return nil
	}))
}
