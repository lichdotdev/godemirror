package main

import (
	"log"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lichdotdev/godemirror"
)

// The scripted edit the page plays when the demo scrolls into view: a line
// typed a character at a time, then every sku in ServeHTTP selected at once
// and renamed by typing.
const (
	keyDelay   = 55 * time.Millisecond
	stepPause  = 700 * time.Millisecond
	typedLine  = "\n\tw.Header().Set(\"Cache-Control\", \"no-store\")"
	renamedTo  = "id"
	anchorLine = "w.Header().Set(\"Content-Type\", \"application/json\")"
)

var skuWord = regexp.MustCompile(`\bsku\b`)

// interrupted stops the script: the user pressed or typed in the demo, and
// the script would go on moving the cursor and typing under them.
var interrupted atomic.Bool

// play runs the script on the file as it opened, each step sent to the UI
// goroutine through do, in order, until interrupted.
func play(send func(func(*demo))) {
	interrupted.Store(false)
	do := func(f func(*demo)) {
		if !interrupted.Load() {
			send(func(d *demo) {
				if !interrupted.Load() {
					f(d)
				}
			})
		}
	}
	do(func(d *demo) {
		d.showDiff = false
		logErr(d.editor.SetState(editorConfig()))
		doc := d.editor.State().Doc()
		at := strings.Index(doc, anchorLine)
		if at < 0 {
			return
		}
		end := at + len(anchorLine)
		logErr(d.editor.Dispatch(godemirror.TransactionSpec{Selection: godemirror.SingleSelection(end, end), ScrollIntoView: true}))
	})
	if !pause(stepPause) {
		return
	}
	for _, c := range typedLine {
		ch := string(c)
		do(func(d *demo) { logErr(d.editor.Dispatch(d.editor.State().ReplaceSelection(ch))) })
		if !pause(keyDelay) {
			return
		}
	}
	if !pause(stepPause) {
		return
	}
	do(func(d *demo) {
		doc := d.editor.State().Doc()
		from := strings.Index(doc, "func (s *Store) ServeHTTP")
		to := strings.Index(doc[from:], "\n}") + from
		var ranges []godemirror.SelectionRange
		for _, m := range skuWord.FindAllStringIndex(doc[from:to], -1) {
			ranges = append(ranges, godemirror.Range(from+m[0], from+m[1]))
		}
		if len(ranges) > 0 {
			logErr(d.editor.Dispatch(godemirror.TransactionSpec{Selection: godemirror.NewSelection(ranges, 0)}))
		}
	})
	if !pause(stepPause * 2) {
		return
	}
	for _, c := range renamedTo {
		ch := string(c)
		do(func(d *demo) { logErr(d.editor.Dispatch(d.editor.State().ReplaceSelection(ch))) })
		if !pause(keyDelay * 3) {
			return
		}
	}
}

// pause waits d, and reports whether the script goes on.
func pause(d time.Duration) bool {
	time.Sleep(d)
	return !interrupted.Load()
}

func logErr(err error) {
	if err != nil {
		log.Print(err)
	}
}
