package glyphs

import (
	"os"
	"path/filepath"
)

const usesFontconfig = false

var (
	monospaceFamilies = []string{"Menlo", "Monaco", "Courier New"}
	fallbackFamilies  = []string{"PingFang SC", "Hiragino Sans", "Apple SD Gothic Neo", "Apple Symbols"}
)

func fontDirs() []string {
	dirs := []string{"/System/Library/Fonts", "/Library/Fonts"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
	}
	return dirs
}
