//go:build js

package main

import "syscall/js"

// markFirstFrame tells the page the first frame is drawn, for the time to
// first frame it reports.
func markFirstFrame() {
	js.Global().Set("godemirrorFirstFrame", js.Global().Get("performance").Call("now"))
}
