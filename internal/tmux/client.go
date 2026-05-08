// Package tmux is a thin os/exec wrapper around the tmux binary. It does not
// speak control mode — research showed sub-millisecond savings for one-shot
// launchers (see docs/research/tmux-login/scripting/03-control-mode-and-ipc.md).
// Every helper batches into one tmux call where possible.
package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

type Client struct {
	bin string
	env []string
}

// New returns a client. If the TMUX_BIN env var is set, that path is used;
// otherwise PATH lookup. Tests inject TMUX_BIN to point at a stub.
func New() *Client {
	bin := os.Getenv("TMUX_BIN")
	if bin == "" {
		bin = "tmux"
	}
	// LANG=C keeps `tmux -V` and friends parseable across locales.
	env := append(os.Environ(), "LANG=C", "LC_ALL=C")
	return &Client{bin: bin, env: env}
}

// Cmd builds an exec.Cmd with the configured binary, env, and stderr wired
// through so tmux errors are visible. Stdout is left for the caller to capture.
func (c *Client) Cmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Env = c.env
	cmd.Stderr = os.Stderr
	return cmd
}

// Output runs tmux with args and returns stdout. Uses a captured stderr buffer
// so errors carry the tmux message; if tmux exits non-zero, the error message
// is the stderr content (helpful for "no server running" etc.).
func (c *Client) Output(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Env = c.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("tmux %v: %w (stderr=%s)", args, err, bytes.TrimSpace(stderr.Bytes()))
	}
	return stdout.Bytes(), nil
}

// Run runs tmux with args, ignoring stdout but propagating stderr. Used for
// fire-and-forget commands (kill-session, rename-session, etc.).
func (c *Client) Run(ctx context.Context, args ...string) error {
	cmd := c.Cmd(ctx, args...)
	cmd.Stdout = nil // discard
	return cmd.Run()
}

// IsNoServer reports whether err looks like "no server running" — tmux exits
// 1 with that on stderr when no server is up. Used by callers that want to
// degrade silently instead of erroring (e.g. session list at login time).
func IsNoServer(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "no server running") || contains(msg, "error connecting")
}

func contains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}

// errNoServer is returned by helpers that explicitly degrade on a missing
// server, so callers can distinguish "tmux isn't running yet" from real errors.
var errNoServer = errors.New("tmux: no server running")
