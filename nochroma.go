//go:build godemirror_nochroma

package godemirror

// highlightOther is nil with the godemirror_nochroma tag, which leaves out
// chroma and its lexers (about 8 MB of a WebAssembly build): a file no Lezer
// grammar or stream mode reads shows plain.
func highlightOther(string, string) (*document, error) { return nil, nil }
