package godemirror

import (
	"context"

	"gioui.org/layout"
	"gioui.org/op"
)

// call is a caller's hook (Complete, Hover, Definition) running off the UI
// goroutine. Layout looks for its answer each frame, as it looks for
// highlighting, and asks for frames until it comes.
type call[T any] struct {
	cancel context.CancelFunc
	done   chan T
}

// startCall runs f on a goroutine of its own, with a context stop cancels.
func startCall[T any](f func(ctx context.Context) T) *call[T] {
	ctx, cancel := context.WithCancel(context.Background())
	c := &call[T]{cancel: cancel, done: make(chan T, 1)}
	go func() { c.done <- f(ctx) }()
	return c
}

// poll is the call's answer, ok false while it has none; then it asks for
// another frame to look again.
func (c *call[T]) poll(gtx layout.Context) (v T, ok bool) {
	select {
	case v = <-c.done:
		c.cancel()
		return v, true
	default:
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(highlightPoll)})
		return v, false
	}
}

// stop cancels the call, whose answer no one will read; a nil call is
// already stopped.
func (c *call[T]) stop() {
	if c != nil {
		c.cancel()
	}
}
