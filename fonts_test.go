package godemirror

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"runtime"
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

// installFonts puts fonts' files in the current user's font directory, which
// is a fresh one for the rest of the test.
func installFonts(t *testing.T, fonts Fonts) {
	t.Helper()
	home := t.TempDir()
	var dir string
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", home)
		dir = filepath.Join(home, "Microsoft", "Windows", "Fonts")
	case "darwin":
		t.Setenv("HOME", home)
		dir = filepath.Join(home, "Library", "Fonts")
	default:
		t.Setenv("XDG_DATA_HOME", home)
		dir = filepath.Join(home, "fonts")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"Regular.ttf": fonts.Regular, "Bold.ttf": fonts.Bold,
		"Italic.ttf": fonts.Italic, "BoldItalic.ttf": fonts.BoldItalic,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewWithoutFontconfigFindsAnInstalledFamily(t *testing.T) {
	withoutFontconfig(t)
	installFonts(t, goMono)
	v, err := New("Go Mono", 12)
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

func TestNewWithoutFontconfigIsErrFontNotFoundForAFamilyNotInstalled(t *testing.T) {
	withoutFontconfig(t)
	installFonts(t, goMono)
	_, err := New("No Such Family", 12)
	if !errors.Is(err, ErrFontNotFound) {
		t.Fatalf("err = %v, want ErrFontNotFound", err)
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
