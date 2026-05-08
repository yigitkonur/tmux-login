package tmux

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

// AttachSpec describes the desired session-attach. Name is required; Cwd is
// the new-session start directory if the session doesn't exist yet.
type AttachSpec struct {
	Name string
	Cwd  string
	// Detach skips the final attach/switch-client (used for scripting).
	Detach bool
}

// Attach implements the canonical idempotent attach idiom:
//
//	tmux new-session -A -d -s NAME -c DIR        (idempotent: creates if absent)
//	tmux switch-client -t =NAME                  (if $TMUX is set)
//	tmux attach -t =NAME                         (if not inside tmux; via execve)
//
// `-A` makes new-session attach to an existing session of the same name
// instead of erroring. `-d` keeps it detached so we can branch deterministically
// into either switch-client or attach. The `=NAME` target syntax disables
// fnmatch wildcards, so a session named e.g. `foo*` doesn't accidentally match.
//
// On the attach branch this exec's into tmux so the user's shell becomes the
// tmux client. On the switch-client branch it returns normally because the
// calling tmux client is being switched in place.
func (c *Client) Attach(ctx context.Context, spec AttachSpec) error {
	if spec.Name == "" {
		return os.ErrInvalid
	}

	createArgs := []string{"new-session", "-A", "-d", "-s", spec.Name}
	if spec.Cwd != "" {
		createArgs = append(createArgs, "-c", spec.Cwd)
	}
	if err := c.Run(ctx, createArgs...); err != nil {
		return err
	}

	if spec.Detach {
		return nil
	}

	if InsideTmux() {
		return c.Run(ctx, "switch-client", "-t", "="+spec.Name)
	}
	return c.execAttach(spec.Name)
}

func (c *Client) execAttach(name string) error {
	bin, err := exec.LookPath(c.bin)
	if err != nil {
		return err
	}
	args := []string{c.bin, "attach", "-t", "=" + name}
	return syscall.Exec(bin, args, c.env)
}
