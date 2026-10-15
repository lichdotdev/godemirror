package shell_test

import (
	"testing"

	"github.com/lichdotdev/godemirror/internal/stream/shell"
	"github.com/lichdotdev/godemirror/internal/stream/streamtest"
)

// TestMatchesCodeMirror highlights every fixture and requires the spans
// @codemirror/legacy-modes' shell gave it through StreamLanguage, as
// internal/stream/gen/golden.mjs wrote them down.
func TestMatchesCodeMirror(t *testing.T) {
	streamtest.MatchGoldens(t, shell.New, "testdata")
}
