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
// When we *just created* a session (HasSession=false before the call), we also
// send-keys `cd DIR; clear` to the new pane. tmux's `-c DIR` does set the
// initial cwd, but some user shell-integration setups (cmux relays, terminal
// shell-integration scripts, plugin frameworks that source many rcs) cd
// elsewhere late in shell startup and silently override `-c`. The send-keys
// is a no-op visually on a clean shell (cd to current dir, then clear), and
// fixes the case where the shell did drift. It's never sent for *existing*
// sessions we're just attaching to — that would disturb whatever the user
// has running there.
//
// On the attach branch this exec's into tmux so the user's shell becomes the
// tmux client. On the switch-client branch it returns normally because the
// calling tmux client is being switched in place.
func (c *Client) Attach(ctx context.Context, spec AttachSpec) error {
	if spec.Name == "" {
		return os.ErrInvalid
	}

	wasFresh := !c.HasSession(ctx, spec.Name)

	createArgs := []string{"new-session", "-A", "-d", "-s", spec.Name}
	if spec.Cwd != "" {
		createArgs = append(createArgs, "-c", spec.Cwd)
	}
	if err := c.Run(ctx, createArgs...); err != nil {
		return err
	}

	if wasFresh && spec.Cwd != "" {
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
