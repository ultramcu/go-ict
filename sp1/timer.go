// SPDX-License-Identifier: MIT
// Copyright (c) 2026 MaIII Themd

package sp1

import (
	"sync"
	"time"
)

// elapsedTimer reports how long it has been since its last call to
// SinceLastCall. Used by the write drainer to enforce the SP1's
// "wait before sending the next batch after a CR" rule without having
// to track a separate monotonic clock.
type elapsedTimer struct {
	mu       sync.Mutex
	lastCall time.Time
}

func newElapsedTimer() *elapsedTimer {
	return &elapsedTimer{lastCall: time.Now()}
}

// SinceLastCall returns the duration since the previous SinceLastCall
// (or since construction, on the first call) and updates the internal
// timestamp.
func (t *elapsedTimer) SinceLastCall() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	d := now.Sub(t.lastCall)
	t.lastCall = now
	return d
}
