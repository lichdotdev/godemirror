package godemirror

import (
	"errors"
	"image"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
)

// uncovered is text with an unassigned character, which no font covers.
const uncovered = "a \U00050000 b"

var goMono = Fonts{
	Regular:    gomono.TTF,
	Bold:       gomonobold.TTF,
	Italic:     gomonoitalic.TTF,
	BoldItalic: gomonobolditalic.TTF,
}

// withoutFontconfig hides fc-match and fc-list for the rest of the test.
func withoutFontconfig(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
}

func TestNewWithoutFontconfigIsErrNoFontconfig(t *testing.T) {
	withoutFontconfig(t)
	_, err := New("monospace", 12)
	if !errors.Is(err, ErrNoFontconfig) {
		t.Fatalf("err = %v, want ErrNoFontconfig", err)
	}
}

func TestAFallbackLookupThatFailsDrawsTofu(t *testing.T) {
	v, err := New("monospace", 12)
	if err != nil {
		t.Fatal(err)
	}
	withoutFontconfig(t)
	v.SetContent(Content{Text: uncovered})
	h := &harness{t: t, v: v, size: image.Pt(400, 200), now: time.Unix(1, 0)}
	h.frame()
}

func TestNewFromFontsNeedsNoFontconfig(t *testing.T) {
	withoutFontconfig(t)
	v, err := NewFromFonts(goMono, 12)
	if err != nil {
		t.Fatal(err)
	}
	v.SetContent(Content{Text: uncovered})
	h := &harness{t: t, v: v, size: image.Pt(400, 200), now: time.Unix(1, 0)}
	h.frame()
	h.click(h.at(0, 0), 0)
	h.click(h.at(0, 0), 0)
	if got := v.SelectedText(); got != "a" {
		t.Fatalf("selected %q, want %q", got, "a")
	}
}

func TestNewFromFontsNeedsRegular(t *testing.T) {
	if _, err := NewFromFonts(Fonts{Bold: gomonobold.TTF}, 12); err == nil {
		t.Fatal("no error without a Regular font")
	}
}

func TestNewFromFontsRejectsWhatIsNotAFont(t *testing.T) {
	fonts := goMono
	fonts.Italic = []byte("not a font")
	if _, err := NewFromFonts(fonts, 12); err == nil {
		t.Fatal("no error for an Italic that is not a font")
	}
}
