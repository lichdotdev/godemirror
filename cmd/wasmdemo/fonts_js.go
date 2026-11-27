//go:build js

package main

import (
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"

	"github.com/lichdotdev/godemirror"
)

// fonts are the faces the demo draws code with. A browser has no
// fontconfig and no font directories to scan, so the Go Mono faces are
// built in.
func fonts() (godemirror.Fonts, string) {
	return godemirror.Fonts{
		Regular: gomono.TTF, Bold: gomonobold.TTF, Italic: gomonoitalic.TTF, BoldItalic: gomonobolditalic.TTF,
	}, ""
}
