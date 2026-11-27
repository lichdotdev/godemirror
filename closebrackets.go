package godemirror

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// pairs are the closing characters CloseBrackets types after the opening
// ones, @codemirror/autocomplete's closeBrackets defaults.
var pairs = map[string]string{"(": ")", "[": "]", "{": "}", `"`: `"`, "'": "'", "`": "`"}

// closesBefore are the characters before which an opening one is closed,
// beside a space and the end of the line: closeBrackets' default before.
const closesBefore = ")]}:;>"

// closePair is change, text typed at a single cursor, as CloseBrackets types
// it, and where the cursor goes: an opening character with its closing one,
// or nothing when the closing character typed is already next.
func (s EditorState) closePair(change ChangeSpec) (ChangeSpec, int) {
	end := change.From + len(change.Insert)
	if change.From != change.To || len(s.sel.ranges) != 1 {
		return change, end
	}
	next, _ := utf8.DecodeRuneInString(s.doc[change.From:])
	if change.From < len(s.doc) && string(next) == change.Insert && strings.Contains(`)]}"'`+"`", change.Insert) {
		return ChangeSpec{From: change.From, To: change.From}, end
	}
	closing, ok := pairs[change.Insert]
	if !ok || change.From < len(s.doc) && next != '\n' && !unicode.IsSpace(next) && !strings.ContainsRune(closesBefore, next) {
		return change, end
	}
	if prev, _ := utf8.DecodeLastRuneInString(s.doc[:change.From]); closing == change.Insert && categorize(string(prev)) == categoryWord {
		return change, end
	}
	return ChangeSpec{From: change.From, To: change.To, Insert: change.Insert + closing}, end
}

// deletePair is the transaction Backspace makes between an empty pair, as
// closeBrackets' deleteBracketPair: both characters go.
func (s EditorState) deletePair() (TransactionSpec, bool) {
	if len(s.sel.ranges) != 1 || !s.sel.Main().Empty() {
		return TransactionSpec{}, false
	}
	pos := s.sel.Main().Head()
	if pos == 0 || pos == len(s.doc) || pairs[s.doc[pos-1:pos]] != s.doc[pos:pos+1] {
		return TransactionSpec{}, false
	}
	return TransactionSpec{Changes: []ChangeSpec{{From: pos - 1, To: pos + 1}}, UserEvent: "delete.backward", ScrollIntoView: true}, true
}
