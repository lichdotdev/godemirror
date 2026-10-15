package kotlin_test

import (
	"testing"

	"github.com/lichdotdev/godemirror/internal/stream/kotlin"
	"github.com/lichdotdev/godemirror/internal/stream/streamtest"
)

// TestMatchesCodeMirror highlights every fixture and requires the spans
// @codemirror/legacy-modes' kotlin gave it through StreamLanguage, as
// internal/stream/gen/golden.mjs wrote them down.
func TestMatchesCodeMirror(t *testing.T) {
	streamtest.MatchGoldens(t, kotlin.New, "testdata")
}
