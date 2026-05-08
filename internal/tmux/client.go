// Package tmux is a thin os/exec wrapper around the tmux binary. With sesh
// as the session engine, this package only handles ctrl-k kill (kill-
// session), the type-to-create attach idiom (new-session -d + send-keys
// cwd-lock + execve into tmux attach), and `tmux -V` / `tmux show -gv`
// queries used by the doctor diagnostic.
package tmux

import (
	"bytes"
	"context"
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
// We DO NOT override LANG/LC_ALL. Forcing LC_ALL=C made tmux 3.6 treat
// every non-ASCII byte (including \t in -F format output) as garbage and
// substitute `_`, which then propagates to every pane the server spawns.
// Inherit the user's locale unmodified. (See commit af47aa2 for the bug
// that motivated this comment.)
func New() *Client {
	bin := os.Getenv("TMUX_BIN")
	if bin == "" {
		bin = "tmux"
	}
	return &Client{bin: bin, env: os.Environ()}
}

// Cmd builds an exec.Cmd with the configured binary, env, and stderr wired
// through so tmux errors are visible. Stdout is left for the caller.
func (c *Client) Cmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Env = c.env
	cmd.Stderr = os.Stderr
	return cmd
}

// Output runs tmux with args and returns stdout. Used by Version() and
// the doctor diagnostic for `tmux show -gv …` reads.
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

// Run runs tmux with args, ignoring stdout but propagating stderr. Used
// for fire-and-forget commands (new-session -d, kill-session, send-keys).
func (c *Client) Run(ctx context.Context, args ...string) error {
	cmd := c.Cmd(ctx, args...)
	cmd.Stdout = nil
	return cmd.Run()
}
