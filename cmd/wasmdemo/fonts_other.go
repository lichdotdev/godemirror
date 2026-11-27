//go:build !js

package main

import "github.com/lichdotdev/godemirror"

// fonts are the faces the demo draws code with: off the browser, the
// installed monospace family, resolved as godemirror.New resolves it.
func fonts() (godemirror.Fonts, string) { return godemirror.Fonts{}, "monospace" }
