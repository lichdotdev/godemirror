//go:build !windows && !darwin

package glyphs

import (
	"os"
	"path/filepath"
)

// usesFontconfig: fc-match resolves a family where it is installed, and the
// font directories are scanned where it is not.
const usesFontconfig = true

var (
	monospaceFamilies = []string{"DejaVu Sans Mono", "Noto Sans Mono", "Liberation Mono"}
	fallbackFamilies  = []string{"DejaVu Sans", "Noto Sans CJK SC", "Noto Sans"}
)

// fontDirs are fontconfig's default directories, as the XDG base directory
// spec places the user's.
func fontDirs() []string {
	dirs := []string{"/usr/share/fonts", "/usr/local/share/fonts"}
	dataHome := os.Getenv("XDG_DATA_HOME")
	home, err := os.UserHomeDir()
	if dataHome == "" && err == nil {
		dataHome = filepath.Join(home, ".local", "share")
	}
	if dataHome != "" {
		dirs = append(dirs, filepath.Join(dataHome, "fonts"))
	}
	if err == nil {
		dirs = append(dirs, filepath.Join(home, ".fonts"))
	}
	return dirs
}
