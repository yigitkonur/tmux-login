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
	// ForceCwd sends a cwd-lock command even when the session already exists.
	// Use only for explicit project-directory picker rows.
	ForceCwd bool
	// Detach skips the final attach/switch-client (used for scripting).
	Detach bool
}

// Attach is the idempotent create-or-attach entry point.
//
//	if !has-session NAME:
//	    tmux new-session -d -s NAME -c DIR        (fresh detached create)
//	    tmux send-keys -t =NAME:. 'cd DIR && clear' Enter   (lock cwd against
//	                                                         shell-startup drift)
//	if InsideTmux:
//	    tmux switch-client -t =NAME               (in-place client switch)
//	else:
//	    syscall.Exec tmux attach -t =NAME         (replace ourselves with the
//	                                                client; user's shell becomes
//	                                                the tmux client)
//
// We deliberately do NOT use `tmux new-session -A -d`. On tmux 3.4-3.6a the
// `-A` flag makes tmux switch into attach behavior when the session already
// exists, ignoring `-d` and trying to open a tty. In login contexts where
// stdin isn't a real tty (cmux's broken-ssh-multiplexing fallback, scripted
// invocation, captive subshells), tmux dies with `open terminal failed:
// not a terminal` and the whole attach fails. Splitting the branch around
// has-session avoids the misbehavior entirely; it also makes the "send-keys
// to lock cwd" branch trivially correct (only fires for the fresh-create
// path).
//
// The `=NAME` target syntax disables fnmatch wildcards, so a session named
// e.g. `foo*` doesn't accidentally match other sessions.
func (c *Client) Attach(ctx context.Context, spec AttachSpec) error {
	if spec.Name == "" {
		return os.ErrInvalid
	}

	existed := c.HasSession(ctx, spec.Name)
	if !existed {
		createArgs := []string{"new-session", "-d", "-s", spec.Name}
		if spec.Cwd != "" {
			createArgs = append(createArgs, "-c", spec.Cwd)
		}
		if err := c.Run(ctx, createArgs...); err != nil {
			return err
		}
		if spec.Cwd != "" {
			c.refreshFreshPaneCwd(ctx, spec.Name, spec.Cwd)
		}
	} else if spec.ForceCwd && spec.Cwd != "" {
		c.refreshFreshPaneCwd(ctx, spec.Name, spec.Cwd)
	}

	if spec.Detach {
		return nil
	}

	if InsideTmux() {
		return c.Run(ctx, "switch-client", "-t", "="+spec.Name)
	}
	return c.execAttach(spec.Name)
}

// refreshFreshPaneCwd sends `cd DIR; clear` to a newly-created session's
// active pane, with DIR shell-quoted. Best-effort: errors are swallowed
// because this is a defensive belt-and-braces — the worst case is the
// user's shell ignored `-c` and we couldn't fix it, which is no worse
// than the pre-fix state.
func (c *Client) refreshFreshPaneCwd(ctx context.Context, name, cwd string) {
	target := "=" + name + ":."
	cdLine := "cd " + shellSingleQuote(cwd) + " && clear"
	_ = c.Run(ctx, "send-keys", "-t", target, cdLine, "Enter")
}

// shellSingleQuote wraps s in POSIX single quotes, escaping any embedded
// single quote as the canonical '\” sequence.
func shellSingleQuote(s string) string {
	const sq = "'"
	const escSq = `'\''`
	out := make([]byte, 0, len(s)+2)
	out = append(out, sq...)
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, escSq...)
			continue
		}
		out = append(out, s[i])
	}
	out = append(out, sq...)
	return string(out)
}

func (c *Client) execAttach(name string) error {
	bin, err := exec.LookPath(c.bin)
	if err != nil {
		return err
	}
	args := []string{c.bin, "attach", "-t", "=" + name}
	return syscall.Exec(bin, args, c.env)
}
