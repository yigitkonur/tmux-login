package tmux

import (
	"context"
	"os"
	"strings"
)

// Version returns the tmux version string ("3.6a", "3.4", etc.) parsed
// from `tmux -V`. Empty string if tmux isn't on PATH or the call fails.
// Used by `tmux-login doctor`.
func (c *Client) Version(ctx context.Context) string {
	out, err := c.Output(ctx, "-V")
	if err != nil {
		return ""
	}
	// Format: "tmux 3.6a" or "tmux next-3.6"
	s := strings.TrimSpace(string(out))
	s = strings.TrimPrefix(s, "tmux ")
	return s
}

// InsideTmux reports whether the calling process is inside a tmux client
// (the $TMUX env var is set on every process spawned by a tmux server).
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}
