package glyphs

import (
	"os"
	"path/filepath"
)

const usesFontconfig = false

var (
	monospaceFamilies = []string{"Cascadia Mono", "Consolas", "Courier New"}
	fallbackFamilies  = []string{"Microsoft YaHei", "Yu Gothic", "Malgun Gothic", "Segoe UI Symbol", "Segoe UI Emoji"}
)

// fontDirs are where Windows installs fonts for every user and for the
// current one.
func fontDirs() []string {
	var dirs []string
	if windir := os.Getenv("WINDIR"); windir != "" {
		dirs = append(dirs, filepath.Join(windir, "Fonts"))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
	}
	return dirs
}
