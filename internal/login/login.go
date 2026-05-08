// Package login orchestrates the SSH-login flow. The zsh hook
// (share/login-hook.zsh) gates this with parameter-expansion-only guards
// before exec-ing us; we re-check the cheapest guards as defense-in-depth
// and then drive the picker.
//
// As of v0.3 sesh is required: `sesh list --icons` provides the picker
// items (multi-source: tmux + zoxide + sesh.toml + tmuxinator with Nerd
// Font icons), and `sesh connect NAME` handles idempotent attach. We
// keep our type-to-create + dir picker because sesh has no equivalent
// for "user named a new project; ask where it should live."
package login

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/yigitkonur/tmux-login/internal/cache"
	"github.com/yigitkonur/tmux-login/internal/config"
	"github.com/yigitkonur/tmux-login/internal/perf"
	"github.com/yigitkonur/tmux-login/internal/picker"
	"github.com/yigitkonur/tmux-login/internal/sesh"
	"github.com/yigitkonur/tmux-login/internal/sources"
	"github.com/yigitkonur/tmux-login/internal/tmux"
)

// Run is the entry point for `tmux-login login`. Always returns nil —
// this path must never error out a login. Diagnostic warnings go to stderr.
func Run(ctx context.Context) error {
	cfg := config.Load()
	if cfg.Skip {
		return nil
	}
	if tmux.InsideTmux() {
		return nil
	}
	if err := cfg.EnsureDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "tmux-login: %v\n", err)
		return nil
	}

	tr := perf.New(cfg.StateDir, cfg.Perf)
	defer tr.Close()
	tr.Mark("start")

	c := cache.New(cfg.CacheDir)
	tx := tmux.New()
	sx := sesh.New()

	if !sx.Available() {
		fmt.Fprintln(os.Stderr, "tmux-login: sesh is required but not on PATH — install with 'brew install sesh'")
		return nil
	}

	items, err := buildItems(ctx, sx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmux-login: list: %v\n", err)
	}
	tr.Mark("list_done", "n=", len(items))

	lines := []string{picker.EncodeSkip()}
	for _, it := range items {
		lines = append(lines, picker.Encode(it))
	}

	// Ctrl-K is an in-place kill: fzf executes our hidden `_action`
	// subcommand to kill the highlighted session, then reloads its line
	// list, then drops the cursor on position 2 (first real session
	// after the [skip] sentinel) so chained kills work without the
	// cursor jumping. The picker stays open the whole time — no flicker.
	selfBin := selfBinaryPath()
	binds := []string{
		fmt.Sprintf(
			"ctrl-k:execute-silent(%s _action --kill {})+reload(%s _action --list)+pos(2)",
			selfBin, selfBin,
		),
	}

	header := picker.HeaderFor("login", "enter=attach/create  c-k=kill (in place)  esc=plain shell")
	r, err := picker.Pick(ctx, picker.Spec{
		Prompt:     "tmux session > ",
		Header:     header,
		Lines:      lines,
		Binds:      binds,
		PrintQuery: true,
	}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmux-login: picker: %v\n", err)
		return nil
	}
	tr.Mark("picker_done", "rc=", r.RC)

	if r.IsCancelled() {
		return nil
	}

	parsed := r.Parsed()

	switch {
	case parsed.OK && parsed.Action == picker.SkipSentinel:
		return nil

	case parsed.OK && parsed.Action == "attach":
		if err := sx.Connect(ctx, parsed.Target); err != nil {
			fmt.Fprintf(os.Stderr, "tmux-login: sesh connect: %v\n", err)
		}
		return nil

	case r.IsTypeToCreate():
		dir, err := pickDirectory(ctx, cfg, c, r.Query)
		if err != nil || dir == "" {
			return nil
		}
		spec := tmux.AttachSpec{Name: r.Query, Cwd: dir}
		if err := dispatchAttach(ctx, tx, c, spec); err != nil {
			fmt.Fprintf(os.Stderr, "tmux-login: attach: %v\n", err)
		}
		return nil
	}

	return nil
}

// selfBinaryPath returns the absolute path to the running tmux-login
// binary, with shell-quoting suitable for embedding in fzf bind strings.
// fzf passes the bind value to `sh -c`, so any space or shell-special
// char in the path needs single-quote escaping. Falls back to "tmux-login"
// if exec.LookPath fails (it shouldn't — we're literally running).
func selfBinaryPath() string {
	if exe, err := os.Executable(); err == nil {
		return shellQuoteForBind(exe)
	}
	if path, err := exec.LookPath("tmux-login"); err == nil {
		return shellQuoteForBind(path)
	}
	return "tmux-login"
}

func shellQuoteForBind(s string) string {
	// Single-quoted POSIX string; embedded ' becomes '\''.
	const sq = "'"
	out := make([]byte, 0, len(s)+2)
	out = append(out, sq...)
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\\', '\'', '\'')
			continue
		}
		out = append(out, s[i])
	}
	out = append(out, sq...)
	return string(out)
}

// RunLast jumps to the last-attached session via `sesh last`. Wired to
// `tmux-login last` and to the M-L keybinding in share/tmux.conf.
func RunLast(ctx context.Context) error {
	sx := sesh.New()
	if !sx.Available() {
		fmt.Fprintln(os.Stderr, "tmux-login: `last` requires sesh — install with 'brew install sesh'")
		return nil
	}
	if err := sx.Last(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "tmux-login: sesh last: %v\n", err)
	}
	return nil
}

// buildItems converts sesh's list output into picker items. Display keeps
// the raw ANSI-coloured Nerd-Font-icon-prefixed line so fzf renders it
// with --ansi; Target is what sesh.Connect needs.
func buildItems(ctx context.Context, sx *sesh.Client) ([]sources.Item, error) {
	seshItems, err := sx.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]sources.Item, 0, len(seshItems))
	for _, si := range seshItems {
		out = append(out, sources.Item{
			Mode:       sources.ModeProjects, // not actually projects, but Mode is unused for sesh items
			Display:    si.Display,
			ActionKind: sources.ActionAttach,
			Target:     si.Target,
		})
	}
	return out, nil
}

// dispatchAttach is the type-to-create attach path. tmux owns this one
// because sesh's connect can't take an explicit cwd flag. Records the
// chosen dir in the MRU file so the next dir picker sorts it to the top.
func dispatchAttach(ctx context.Context, tx *tmux.Client, c *cache.Cache, spec tmux.AttachSpec) error {
	if err := tx.Attach(ctx, spec); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	if spec.Cwd != "" {
		_ = c.RecordRecentDir(spec.Cwd)
	}
	return nil
}
