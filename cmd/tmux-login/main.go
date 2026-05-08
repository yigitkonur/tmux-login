// tmux-login dispatches subcommands. Stdlib `flag` only — no Cobra (~30 ms
// cold-start tax). Each subcommand owns a flag.FlagSet so help text stays
// scoped.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yigitkonur/tmux-login/internal/cache"
	"github.com/yigitkonur/tmux-login/internal/config"
	"github.com/yigitkonur/tmux-login/internal/doctor"
	"github.com/yigitkonur/tmux-login/internal/install"
	"github.com/yigitkonur/tmux-login/internal/login"
	"github.com/yigitkonur/tmux-login/internal/tmux"
	"github.com/yigitkonur/tmux-login/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, helpText())
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]
	ctx := context.Background()

	var err error
	switch sub {
	case "pick":
		err = runPick(ctx, args)
	case "attach":
		err = runAttach(ctx, args)
	case "login":
		err = login.Run(ctx)
	case "last":
		err = login.RunLast(ctx)
	case "kill":
		err = runKill(ctx, args)
	case "_action":
		// Hidden — invoked by fzf --bind in the login picker. Not part
		// of the user-facing help text.
		err = login.RunAction(ctx, args)
	case "install-hooks":
		err = runInstallHooks(args)
	case "doctor":
		err = doctor.Run(ctx, os.Stdout)
	case "version":
		fmt.Printf("tmux-login %s (commit %s built %s)\n", version.Version, version.Commit, version.Date)
	case "help", "-h", "--help":
		fmt.Println(helpText())
	default:
		fmt.Fprintf(os.Stderr, "tmux-login: unknown subcommand: %s\n\n%s\n", sub, helpText())
		os.Exit(2)
	}

	if err != nil {
		var ue *usageError
		if errors.As(err, &ue) {
			fmt.Fprintf(os.Stderr, "tmux-login: %s\n", ue.msg)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "tmux-login: %v\n", err)
		os.Exit(1)
	}
}

type usageError struct{ msg string }

func (u *usageError) Error() string { return u.msg }

// runPick — for v0.1 this is sessions-only, same shape as `login` minus
// the silent-on-bail behavior of the SSH hot path. Reuses login.Run() so
// flow stays in one place.
func runPick(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pick", flag.ContinueOnError)
	mode := fs.String("mode", "sessions", "picker mode (v0.1: sessions only)")
	noPopup := fs.Bool("no-popup", false, "skip display-popup wrapping (already in popup, or not in tmux)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = mode    // v0.1 ignores; v0.2 dispatches modes
	_ = noPopup // v0.1 always inline (popup wrapping is wired in tmux.conf binding)

	return login.Run(ctx)
}

func runAttach(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "starting directory if the session needs to be created")
	detach := fs.Bool("detach", false, "create-or-noop only; do not attach/switch")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return &usageError{msg: "attach: session name required (usage: attach NAME [--cwd DIR] [--detach])"}
	}
	name := fs.Arg(0)
	if name == "" {
		return &usageError{msg: "attach: empty session name"}
	}

	cfg := config.Load()
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}
	c := cache.New(cfg.CacheDir)
	tx := tmux.New()

	spec := tmux.AttachSpec{Name: name, Cwd: *cwd, Detach: *detach}
	if err := tx.Attach(ctx, spec); err != nil {
		return err
	}
	if *cwd != "" {
		_ = c.RecordRecentDir(*cwd)
	}
	return nil
}

func runKill(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("kill", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return &usageError{msg: "kill: session name required"}
	}
	name := fs.Arg(0)

	tx := tmux.New()
	if err := tx.KillSession(ctx, name); err != nil {
		fmt.Fprintf(os.Stderr, "tmux-login: kill: %v\n", err)
	}
	return nil
}

func runInstallHooks(args []string) error {
	fs := flag.NewFlagSet("install-hooks", flag.ContinueOnError)
	doTmux := fs.Bool("tmux", false, "wire ~/.tmux.conf marker block")
	doZsh := fs.Bool("zsh", false, "wire ~/.zshrc marker block")
	prefix := fs.String("prefix", "", "install prefix (default $XDG_DATA_HOME/tmux-login)")
	dryRun := fs.Bool("dry-run", false, "print the diff; do not write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*doTmux && !*doZsh {
		return &usageError{msg: "install-hooks: --tmux and/or --zsh required"}
	}

	cfg := config.Load()
	if *prefix == "" {
		*prefix = cfg.DataDir
	}

	tmuxConfPath := filepath.Join(cfg.Home, install.TmuxConfBasename)
	zshrcPath := filepath.Join(cfg.Home, install.ZshrcBasename)

	tmuxLines := []string{
		"source-file -q " + install.QuoteShell(filepath.Join(*prefix, install.ManagedTmuxConfRel)),
	}
	zshLines := []string{
		fmt.Sprintf("[ -r %s ] && source %s",
			install.QuoteShell(filepath.Join(*prefix, install.ManagedLoginHookRel)),
			install.QuoteShell(filepath.Join(*prefix, install.ManagedLoginHookRel))),
	}

	if *dryRun {
		if *doTmux {
			out, err := install.Diff(tmuxConfPath, tmuxLines)
			if err != nil {
				return err
			}
			fmt.Print(out)
		}
		if *doZsh {
			out, err := install.Diff(zshrcPath, zshLines)
			if err != nil {
				return err
			}
			fmt.Print(out)
		}
		return nil
	}

	if *doTmux {
		added, err := install.EnsureMarker(tmuxConfPath, tmuxLines)
		if err != nil {
			return err
		}
		fmt.Printf("tmux-login: %s — %s\n", tmuxConfPath, statusWord(added))
	}
	if *doZsh {
		added, err := install.EnsureMarker(zshrcPath, zshLines)
		if err != nil {
			return err
		}
		fmt.Printf("tmux-login: %s — %s\n", zshrcPath, statusWord(added))
	}
	return nil
}

func statusWord(added bool) string {
	if added {
		return "marker block added"
	}
	return "already wired (no changes)"
}

func helpText() string {
	return `tmux-login — menu-first tmux launcher

usage:
  tmux-login pick [--mode sessions] [--no-popup]
  tmux-login attach NAME [--cwd DIR] [--detach]
  tmux-login login                       # called by the zsh hook on SSH login
  tmux-login last                        # jump to the previous session (via sesh)
  tmux-login kill NAME
  tmux-login install-hooks --tmux|--zsh [--prefix PATH] [--dry-run]
  tmux-login doctor
  tmux-login version

env vars:
  TMUX_LOGIN_ROOTS    colon-separated dirs walked by the projects picker
  TMUX_LOGIN_SKIP=1   bypass the SSH-login hook for one shell
  TMUX_LOGIN_PERF=1   enable per-event tracer in $XDG_STATE_HOME/tmux-login/perf.log
  TMUX_LOGIN_PRUNE    extra basenames to skip during the project walk
  TMUX_BIN            path to tmux binary (default: PATH lookup)`
}
