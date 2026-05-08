package tmux

import (
	"context"
	"os"
	"strings"
)

// Version returns the tmux version string ("3.6a", "3.4", etc.) parsed from
// `tmux -V`. Empty string if tmux isn't on PATH.
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

// InsideTmux reports whether the calling process is inside a tmux client (the
// $TMUX env var is set by tmux on every spawned process).
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// HasServer pings the server with `tmux has-session`. Returns false (no err)
// when the server isn't running; returns true on success; returns an error
// only for unexpected failures.
func (c *Client) HasServer(ctx context.Context) bool {
	cmd := c.Cmd(ctx, "list-sessions", "-F", "#{session_name}")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}
