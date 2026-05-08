// Package perf is an opt-in event tracer mirroring the zellij-login pattern:
// gated on TMUX_LOGIN_PERF=1 or a sentinel file, append per-event timings to
// $XDG_STATE_HOME/tmux-login/perf.log. The default path is a no-op closure so
// the cold path stays fast.
package perf

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Tracer struct {
	enabled bool
	start   time.Time
	mu      sync.Mutex
	out     *os.File
}

// New returns an enabled tracer if perf is on, or a disabled stub. Disabled
// tracers cost nothing per Mark call (single bool check + return).
func New(stateDir string, enabled bool) *Tracer {
	t := &Tracer{enabled: enabled, start: time.Now()}
	if !enabled {
		return t
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		// Disable on failure rather than spamming stderr on the hot path.
		t.enabled = false
		return t
	}
	f, err := os.OpenFile(filepath.Join(stateDir, "perf.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.enabled = false
		return t
	}
	t.out = f
	fmt.Fprintf(f, "=== %s pid=%d ===\n", time.Now().UTC().Format(time.RFC3339), os.Getpid())
	return t
}

func (t *Tracer) Mark(event string, detail ...any) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	ms := float64(time.Since(t.start).Microseconds()) / 1000.0
	if len(detail) > 0 {
		fmt.Fprintf(t.out, "%8.1fms  %-18s  %s\n", ms, event, fmt.Sprint(detail...))
	} else {
		fmt.Fprintf(t.out, "%8.1fms  %-18s\n", ms, event)
	}
}

func (t *Tracer) Close() {
	if t.out != nil {
		_ = t.out.Close()
	}
}
