//go:build js

package main

import "syscall/js"

// markFirstFrame tells the page the first frame is drawn, for the time to
// first frame it reports.
func markFirstFrame() {
	g := js.Global()
	g.Set("godemirrorFirstFrame", g.Get("performance").Call("now"))
	if ready := g.Get("godemirrorReady"); ready.Type() == js.TypeFunction {
		// Off the frame loop: the page's answer calls back into the demo.
		go ready.Invoke()
	}
}
