// Package sesh wraps the joshmedeski/sesh CLI when it's on PATH. We use sesh
// as the session-list source ("sesh list --icons") and as the idempotent
// attach engine ("sesh connect NAME"). When sesh isn't available, the login
// flow falls back to internal/sources/sessions + internal/tmux/attach.
//
// Design mirrors internal/tmux/client.go: the Client wraps os/exec, inherits
// os.Environ() unmodified (NEVER force LANG=C — that bug bit us hard in
// commit af47aa2 by mangling tmux UTF-8 server-wide).
package sesh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type Client struct {
	bin       string
	available *bool
	availOnce sync.Once
}

// New returns a sesh client. SESH_BIN env var overrides PATH lookup
// (mirrors TMUX_BIN — the runtime test stubs use this hook). When SESH_BIN
// is unset and sesh is not on PATH, Available() returns false and the
// caller should fall back to the no-sesh path.
func New() *Client {
	bin := os.Getenv("SESH_BIN")
	if bin == "" {
		bin = "sesh"
	}
	return &Client{bin: bin}
}

// Available reports whether sesh is callable. Result is cached for the
// lifetime of the Client so the LookPath cost (~500 µs) isn't paid per
// call. The login flow only constructs one Client per Run().
func (c *Client) Available() bool {
	c.availOnce.Do(func() {
		// If SESH_BIN points at an explicit path (set by tests or by users
		// who installed under a non-PATH prefix), honor it without LookPath.
		if _, err := os.Stat(c.bin); err == nil {
			t := true
			c.available = &t
			return
		}
		_, err := exec.LookPath(c.bin)
		t := err == nil
		c.available = &t
	})
	return c.available != nil && *c.available
}

// Item is one row from `sesh list --icons`. Display is the raw line,
// ANSI-coloured and icon-prefixed; pass it straight to fzf with --ansi.
// Target is the bare name/path that `sesh connect` accepts.
type Item struct {
	Display string
	Target  string
}

// List runs `sesh list --icons` and parses the output into Items. Errors
// from the underlying command surface as the second return; callers fall
// back to the internal sessions source on any error.
func (c *Client) List(ctx context.Context) ([]Item, error) {
	cmd := exec.CommandContext(ctx, c.bin, "list", "--icons")
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sesh list: %w (stderr=%s)", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return parseList(stdout.Bytes()), nil
}

// Connect runs `sesh connect NAME`. When called from outside tmux, sesh
// itself execs into `tmux attach`; we therefore syscall.Exec into sesh so
// our Go process is replaced (the same shape as internal/tmux.execAttach).
// When inside tmux, sesh switches the calling client; that's a quick
// roundtrip and we just call it as a subprocess.
func (c *Client) Connect(ctx context.Context, name string) error {
	if name == "" {
		return os.ErrInvalid
	}
	if os.Getenv("TMUX") != "" {
		// Inside tmux: sesh handles switch-client; subprocess is fine.
		cmd := exec.CommandContext(ctx, c.bin, "connect", name)
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// Outside tmux: replace ourselves with sesh, which in turn execs tmux.
	bin, err := exec.LookPath(c.bin)
	if err != nil {
		// SESH_BIN points at an absolute path (e.g. test stub). Fall
		// through and exec by full path.
		if _, statErr := os.Stat(c.bin); statErr == nil {
			bin = c.bin
		} else {
			return err
		}
	}
	return syscall.Exec(bin, []string{c.bin, "connect", name}, os.Environ())
}

// Last runs `sesh last`. Same exec semantics as Connect — we always replace
// ourselves with sesh when outside tmux.
func (c *Client) Last(ctx context.Context) error {
	if os.Getenv("TMUX") != "" {
		cmd := exec.CommandContext(ctx, c.bin, "last")
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	bin, err := exec.LookPath(c.bin)
	if err != nil {
		if _, statErr := os.Stat(c.bin); statErr == nil {
			bin = c.bin
		} else {
			return err
		}
	}
	return syscall.Exec(bin, []string{c.bin, "last"}, os.Environ())
}
