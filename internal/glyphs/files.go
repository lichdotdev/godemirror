package glyphs

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// fontFile is a parsed font file, the first face of a collection. A file on
// disk is read once per process and shared by every Rasterizer: each view has
// its own, a diff has a view per file, and a fallback such as a CJK collection
// is tens of megabytes. sfnt keeps reading from the file's bytes, and a Font
// is safe to share as long as each caller brings its own Buffer.
type fontFile struct {
	sf *sfnt.Font
}

var fontFiles = struct {
	sync.Mutex
	byPath map[string]*fontFile
}{byPath: map[string]*fontFile{}}

func openFontFile(path string) (*fontFile, error) {
	fontFiles.Lock()
	defer fontFiles.Unlock()
	if f, ok := fontFiles.byPath[path]; ok {
		return f, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	fontFiles.byPath[path] = f
	return f, nil
}

// parse reads a font file, or the first font of a collection.
func parse(data []byte) (*fontFile, error) {
	coll, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	sf, err := coll.Font(0)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return &fontFile{sf: sf}, nil
}
