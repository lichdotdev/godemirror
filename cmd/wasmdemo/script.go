package main

import (
	"log"
	"regexp"
	"strings"
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

// play runs the script, each step sent to the UI goroutine through do, in
// order.
func play(do func(func(*demo))) {
	do(func(d *demo) {
		d.showDiff = false
		doc := d.editor.State().Doc()
		at := strings.Index(doc, anchorLine)
		if at < 0 {
			return
		}
		end := at + len(anchorLine)
		logErr(d.editor.Dispatch(godemirror.TransactionSpec{Selection: godemirror.SingleSelection(end, end), ScrollIntoView: true}))
	})
	time.Sleep(stepPause)
	for _, c := range typedLine {
		ch := string(c)
		do(func(d *demo) { logErr(d.editor.Dispatch(d.editor.State().ReplaceSelection(ch))) })
		time.Sleep(keyDelay)
	}
	time.Sleep(stepPause)
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
	time.Sleep(stepPause * 2)
	for _, c := range renamedTo {
		ch := string(c)
		do(func(d *demo) { logErr(d.editor.Dispatch(d.editor.State().ReplaceSelection(ch))) })
		time.Sleep(keyDelay * 3)
	}
}

func logErr(err error) {
	if err != nil {
		log.Print(err)
	}
}
