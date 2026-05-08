package tmux

import (
	"context"
	"strings"
)

// PopupSpec describes a `display-popup -E` invocation. We use -E so the
// popup auto-closes when the inner command exits.
type PopupSpec struct {
	Width   string // "80%"
	Height  string // "70%"
	Title   string
	Env     map[string]string
	Command []string
}

// DisplayPopup launches a tmux popup running the given command. Caller must
// be inside tmux (`InsideTmux()` true) — otherwise tmux exits with "no
// current target". Returns the popup's exit when -E auto-closes it.
func (c *Client) DisplayPopup(ctx context.Context, spec PopupSpec) error {
	args := []string{"display-popup", "-E"}
	if spec.Width != "" {
		args = append(args, "-w", spec.Width)
	}
	if spec.Height != "" {
		args = append(args, "-h", spec.Height)
	}
	if spec.Title != "" {
		args = append(args, "-T", spec.Title)
	}
	for k, v := range spec.Env {
		args = append(args, "-e", k+"="+v)
	}
	// Arg list ends with the shell command. tmux runs it via the user's
	// default-shell; we collapse Command into a single string so quoting
	// inside the popup doesn't get mangled by tmux's own arg parser.
	args = append(args, "--", strings.Join(spec.Command, " "))
	return c.Run(ctx, args...)
}

// SwitchClient brings target (session, window, or pane) to the calling
// tmux client.
func (c *Client) SwitchClient(ctx context.Context, target string) error {
	return c.Run(ctx, "switch-client", "-t", target)
}

// DetachClient detaches the calling client (M-d).
func (c *Client) DetachClient(ctx context.Context) error {
	return c.Run(ctx, "detach-client")
}
