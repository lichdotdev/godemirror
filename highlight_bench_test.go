package godemirror

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf16"

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

// bigHTML is an HTML page of about benchJavaLines lines, its sections each
// nesting a script and a style sheet, and attributes CSS and JavaScript
// parse, as @codemirror/lang-html nests them.
func bigHTML() string {
	const section = `<section id="s%[1]d" class="card" style="margin: 0 auto; color: #333">
  <h2 onclick="toggle(%[1]d)">Section %[1]d &amp; more</h2>
  <p>Some <b>bold</b> and <i>italic</i> text, a <a href="/x/%[1]d">link</a>.
  <ul>
    <li>one
    <li>two
  </ul>
  <script>
    function toggle%[1]d(id) {
      const el = document.getElementById("s" + id);
      if (!el) return;
      el.classList.toggle("open"); // flip
      for (let i = 0; i < 3; i++) console.log(i, ` + "`item ${i}`" + `);
    }
  </script>
  <style>
    #s%[1]d .card { padding: 4px 8px; border: 1px solid rgba(0, 0, 0, .2); }
    #s%[1]d h2:hover { text-decoration: underline; }
  </style>
</section>
`
	var out strings.Builder
	out.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head><title>Big</title></head>\n<body>\n")
	for i := 0; strings.Count(out.String(), "\n") < benchJavaLines; i++ {
		fmt.Fprintf(&out, section, i)
	}
	out.WriteString("</body>\n</html>\n")
	return out.String()
}

// oneCharEdit is text with a letter typed into the middle of the word
// nearest its middle line, as an editor sends a keystroke.
func oneCharEdit(text string) string {
	at := len(text) / 2
	for !unicode.IsLetter(rune(text[at-1])) || !unicode.IsLetter(rune(text[at])) {
		at++
	}
	return text[:at] + "x" + text[at:]
}

// benchmarkReparse measures, after a one-character edit to text, the
// reparse alone and the whole new document, done incrementally from the
// parse before the edit and from scratch.
func benchmarkReparse(b *testing.B, filename, text string) {
	text, edited := normalize(text), normalize(oneCharEdit(text))
	parser := lezerParsers[filepath.Ext(filename)]()
	prev := &lezerParse{parser: parser, units: utf16.Encode([]rune(text))}
	prev.tree = parser.ParseUnits(prev.units, nil, nil)
	units := utf16.Encode([]rune(edited))
	b.Run("parse/incremental", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			reparse(parser, units, prev)
		}
	})
	b.Run("parse/full", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			reparse(parser, units, nil)
		}
	})
	b.Run("document/incremental", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			highlightLezer(parser, edited, prev)
		}
	})
	b.Run("document/full", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			highlightLezer(parser, edited, nil)
		}
	})
}

func BenchmarkReparseJava(b *testing.B) { benchmarkReparse(b, "Big.java", bigJava(b)) }

func BenchmarkReparseHTML(b *testing.B) { benchmarkReparse(b, "big.html", bigHTML()) }
