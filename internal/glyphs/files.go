package glyphs

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// fontFile is a parsed font file, or one face of a collection. A font on
// disk is read once per process and shared by every Rasterizer: each view has
// its own, a diff has a view per file, and a fallback such as a CJK collection
// is tens of megabytes. sfnt keeps reading from the file's bytes, and a Font
// is safe to share as long as each caller brings its own Buffer.
type fontFile struct {
	sf *sfnt.Font
}

// fontRef names one font on disk: index picks it in a collection, and is 0
// for a file holding a single font.
type fontRef struct {
	path  string
	index int
}

var fontFiles = struct {
	sync.Mutex
	byRef map[fontRef]*fontFile
}{byRef: map[fontRef]*fontFile{}}

func openFontFile(ref fontRef) (*fontFile, error) {
	fontFiles.Lock()
	defer fontFiles.Unlock()
	if f, ok := fontFiles.byRef[ref]; ok {
		return f, nil
	}
	data, err := os.ReadFile(ref.path)
	if err != nil {
		return nil, err
	}
	f, err := parse(data, ref.index)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ref.path, err)
	}
	fontFiles.byRef[ref] = f
	return f, nil
}

// parse reads a font file, or the font at index in a collection.
func parse(data []byte, index int) (*fontFile, error) {
	coll, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	sf, err := coll.Font(index)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return &fontFile{sf: sf}, nil
}
