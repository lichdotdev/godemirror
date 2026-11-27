//go:build !js

package main

// listen has no page to listen to off the browser.
func listen(func(func(*demo))) {}
