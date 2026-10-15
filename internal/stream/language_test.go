package stream_test

import (
	"testing"

	"github.com/lichdotdev/godemirror/internal/stream"
	"github.com/lichdotdev/godemirror/internal/stream/streamtest"
)

// names is gen/names.mjs: a line is tokens ended by "/" or the end of the
// line, each token's text (less its "/") its style.
type names struct{}

func (names) Token(s *stream.Stream) string {
	var style []rune
	for ch := s.Next(); ch != stream.EOL && ch != '/'; ch = s.Next() {
		style = append(style, rune(ch))
	}
	return string(style)
}

// TestTokenTableMatchesCodeMirror requires the tags, and the merging of
// touching tokens, that StreamLanguage and highlightTree gave each style
// in testdata/names.txt: legacy names, tags with modifiers, several tags,
// and the parts CodeMirror warns about and skips.
func TestTokenTableMatchesCodeMirror(t *testing.T) {
	streamtest.MatchGoldens(t, func() stream.Mode { return names{} }, "testdata")
}

// stuck reads nothing, ever.
type stuck struct{}

func (stuck) Token(*stream.Stream) string { return "keyword" }

func TestHighlightPanicsOnAModeThatDoesNotAdvance(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Highlight returned for a mode that never reads")
		}
	}()
	streamtest.Describe(stuck{}, "x")
}
