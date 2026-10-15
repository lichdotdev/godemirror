package godemirror

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/lexers"
)

// benchJavaLines is the size of the big file the ceilings measure.
const benchJavaLines = 13000

// bigJava is the Java fixtures @lezer/java parses without an error,
// repeated, each copy's names numbered and its package and imports dropped
// after the first, to about benchJavaLines lines of one file.
func bigJava(b *testing.B) string {
	b.Helper()
	var parts []string
	for _, name := range []string{"Generics.java", "Annotations.java"} {
		src, err := os.ReadFile("internal/lezer/java/testdata/" + name)
		if err != nil {
			b.Fatal(err)
		}
		parts = append(parts, string(src))
	}
	var out strings.Builder
	for i := 0; strings.Count(out.String(), "\n") < benchJavaLines; i++ {
		for _, part := range parts {
			for line := range strings.Lines(part) {
				if i > 0 && (strings.HasPrefix(line, "package ") || strings.HasPrefix(line, "import ")) {
					continue
				}
				out.WriteString(strings.ReplaceAll(line, "LruCache", fmt.Sprint("LruCache", i)))
			}
		}
	}
	return out.String()
}

func BenchmarkHighlightJavaLezer(b *testing.B) {
	text := bigJava(b)
	for b.Loop() {
		if _, err := highlight("Big.java", text); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHighlightJavaChroma(b *testing.B) {
	text := normalize(bigJava(b))
	lexer := lexers.Get("java")
	for b.Loop() {
		if _, err := highlightChroma(lexer, text); err != nil {
			b.Fatal(err)
		}
	}
}
