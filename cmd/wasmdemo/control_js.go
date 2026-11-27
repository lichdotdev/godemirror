//go:build js

package main

import "syscall/js"

// listen lets the page drive the demo: godemirrorShow("editor" or "diff"),
// godemirrorSplit(bool), godemirrorTheme(index), godemirrorPlay() for the
// scripted edit, and godemirrorEmbed() to hide the demo's own buttons. The
// page hears they are there through godemirrorReady, after the first frame.
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
	g.Set("godemirrorPlay", js.FuncOf(func(js.Value, []js.Value) any {
		go play(send)
		return nil
	}))
	g.Set("godemirrorEmbed", js.FuncOf(func(js.Value, []js.Value) any {
		do(func(d *demo) { d.embedded = true })
		return nil
	}))
}
