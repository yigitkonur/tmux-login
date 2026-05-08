// action.go implements the hidden `_action` subcommand. fzf's `--bind`
// invokes it from inside the picker (execute-silent / reload) so the
// picker stays open and re-renders in place when you press Ctrl-X. This
// avoids the flicker of the picker exiting and re-spawning that the
// older --expect=ctrl-x path caused.
//
// Subcommands (all read-only argv records — no user-facing output):
//
//	tmux-login _action --kill <encoded-line>     kill the session if the
//	                                              line is a tmux session;
//	                                              no-op for zoxide rows.
//	tmux-login _action --list                    emit the current picker
//	                                              line list (sesh.List
//	                                              piped through our
//	                                              encoder); fzf reads
//	                                              this for `reload(...)`.
package login

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yigitkonur/tmux-login/internal/picker"
	"github.com/yigitkonur/tmux-login/internal/sesh"
	"github.com/yigitkonur/tmux-login/internal/sources"
	"github.com/yigitkonur/tmux-login/internal/tmux"
)

// RunAction handles `tmux-login _action <flags>`.
func RunAction(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("_action", flag.ContinueOnError)
	doKill := fs.Bool("kill", false, "kill the session encoded on stdin/argv (no-op for zoxide rows)")
	doList := fs.Bool("list", false, "emit the current picker line list (for fzf reload)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch {
	case *doList:
		return emitList(ctx)
	case *doKill:
		return killHighlighted(ctx, fs.Args())
	default:
		return fmt.Errorf("_action: pass --kill or --list")
	}
}

func emitList(ctx context.Context) error {
	sx := sesh.New()
	if !sx.Available() {
		return nil
	}
	items, err := sx.List(ctx)
	if err != nil {
		return nil // soft-fail: empty list is fine for fzf reload
	}
	out := make([]string, 0, len(items)+1)
	out = append(out, picker.EncodeSkip())
	for _, si := range items {
		out = append(out, picker.Encode(sources.Item{
			Mode:       sources.ModeProjects,
			Display:    si.Display,
			ActionKind: sources.ActionAttach,
			Target:     si.Target,
		}))
	}
	fmt.Println(strings.Join(out, "\n"))
	return nil
}

// killHighlighted decodes the picker line passed by fzf's `{}` placeholder
// and kills the session if it's a real tmux session. Sentinels and
// zoxide rows are silently no-ops.
//
// fzf's `{}` expands to the entire selected line including our encoded
// prefix; we feed it as a single argv element. (When the line contains
// shell-special chars they're already escaped by fzf's exec layer.)
func killHighlighted(ctx context.Context, rest []string) error {
	if len(rest) == 0 {
		return nil
	}
	line := strings.Join(rest, " ")
	action, target, _, _, ok := picker.Decode(line)
	if !ok {
		return nil
	}
	if action != "attach" || target == "" {
		return nil
	}
	tx := tmux.New()
	if !tx.HasSession(ctx, target) {
		return nil // zoxide path or stale entry — silently skip
	}
	if err := tx.KillSession(ctx, target); err != nil {
		fmt.Fprintf(os.Stderr, "tmux-login: kill %q: %v\n", target, err)
	}
	return nil
}
