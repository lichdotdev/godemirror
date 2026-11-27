package main

import (
	"testing"
	"time"
)

func TestAnInterruptedPlayStopsSendingSteps(t *testing.T) {
	steps := 0
	send := func(func(*demo)) {
		steps++
		interrupted.Store(true) // the user pressed after the first step
	}
	start := time.Now()
	play(send)
	if steps != 1 {
		t.Fatalf("%d steps sent, want the first only", steps)
	}
	if took := time.Since(start); took > 2*stepPause {
		t.Fatalf("play ran %v after being interrupted", took)
	}
}
