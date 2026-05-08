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
//
// We DO NOT override LANG/LC_ALL here. Originally we set LC_ALL=C "to keep
// tmux -V parseable across locales" — that was overcautious. The real cost
// is that tmux 3.6 treats LC_ALL=C as "single-byte ASCII only" and sanitizes
// every non-ASCII byte (including \t in -F format output) to `_`. Worse,
// the locale propagates to the tmux SERVER process — so every pane the
// server later spawns inherits the broken locale, mangling UTF-8 glyphs in
// every app the user runs (Claude Code's progress bars, vim's box drawing,
// anything with non-ASCII output renders as `___`).
//
// Inherit whatever locale the user has. If they have no locale set, tmux
// falls back internally to a sane default. Format output uses real \t bytes
// regardless.
func New() *Client {
	bin := os.Getenv("TMUX_BIN")
	if bin == "" {
		bin = "tmux"
	}
	return &Client{bin: bin, env: os.Environ()}
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
