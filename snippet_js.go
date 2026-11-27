//go:build js

package godemirror

// Gio's browser text input (gioui.org v0.10.3, app/os_js.go) keeps the
// snippet as its textarea's value, and at compositionend takes that whole
// value for the composed text: a dead key (` on ABNT2, then a letter)
// inserted the text around the caret again and moved the caret past it. In a
// browser the editor gives the text input no text around the caret, so the
// textarea holds only what is being composed.
func init() { snippetAroundCaret = false }
