// Package login orchestrates the SSH-login flow. The zsh hook (share/login-hook.zsh)
// gates this with parameter-expansion-only guards before exec-ing us; we
// re-check the cheapest guards as defense-in-depth and then drive the picker.
package login

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/yigitkonur/tmux-login/internal/cache"
	"github.com/yigitkonur/tmux-login/internal/config"
	"github.com/yigitkonur/tmux-login/internal/perf"
	"github.com/yigitkonur/tmux-login/internal/picker"
	"github.com/yigitkonur/tmux-login/internal/sesh"
	"github.com/yigitkonur/tmux-login/internal/sources"
	"github.com/yigitkonur/tmux-login/internal/tmux"
)

// Run is the entry point for `tmux-login login`. Always returns nil — this
// path must never error a login. Diagnostic warnings go to stderr.
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
	useSesh := sx.Available()
	tr.Mark("engine", "sesh=", useSesh)

	// Loop so ctrl-x can kill the highlighted session and re-render the
	// list. Every other terminal action (attach, type-to-create, skip,
	// cancel) returns immediately.
	for {
		sessions, err := buildSessionItems(ctx, useSesh, sx, tx, c)
		if err != nil {
			// Soft-fail: present an empty list rather than erroring out the login.
			fmt.Fprintf(os.Stderr, "tmux-login: list: %v\n", err)
		}
		tr.Mark("list_done", "n=", len(sessions))

		// Phantom-cache GC. Only meaningful for the no-sesh path (sesh manages
		// its own state); skip when sesh is active to avoid GC'ing entries
		// keyed by sesh's path-style targets.
		if !useSesh {
			liveNames := make([]string, 0, len(sessions))
			for _, s := range sessions {
				liveNames = append(liveNames, s.Target)
			}
			c.GC(liveNames, err == nil)
		}

		lines := []string{picker.EncodeSkip()}
		for _, it := range sessions {
			lines = append(lines, picker.Encode(it))
		}

		header := picker.HeaderFor("login", "enter=attach/create  c-x=kill  esc=plain shell")
		r, err := picker.Pick(ctx, picker.Spec{
			Prompt:     "tmux session > ",
			Header:     header,
			Lines:      lines,
			Expect:     []string{"ctrl-x"},
			PrintQuery: true,
		}, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tmux-login: picker: %v\n", err)
			return nil
		}
		tr.Mark("picker_done", "rc=", r.RC, " key=", r.Key)

		if r.IsCancelled() {
			return nil
		}

		parsed := r.Parsed()

		// Ctrl-X: kill the highlighted session, then re-render. Always uses
		// tmux directly — sesh has no kill subcommand. No-op when the
		// cursor is on the [skip] sentinel or a non-tmux entry (a zoxide
		// path that hasn't been promoted to a session yet).
		if r.Key == "ctrl-x" {
			if parsed.OK && parsed.Action == "attach" && tx.HasSession(ctx, parsed.Target) {
				if killErr := tx.KillSession(ctx, parsed.Target); killErr != nil {
					fmt.Fprintf(os.Stderr, "tmux-login: kill %q: %v\n", parsed.Target, killErr)
				}
				c.DropSession(parsed.Target)
				tr.Mark("killed", parsed.Target)
			}
			continue
		}

		switch {
		case parsed.OK && parsed.Action == picker.SkipSentinel:
			return nil

		case parsed.OK && parsed.Action == "attach":
			// sesh handles idempotent connect (creates if missing, attaches
			// if existing, resolves zoxide paths to session names). For the
			// no-sesh path we use the existing tmux.AttachSpec flow.
			if useSesh {
				if err := sx.Connect(ctx, parsed.Target); err != nil {
					fmt.Fprintf(os.Stderr, "tmux-login: sesh connect: %v\n", err)
				}
				return nil
			}
			spec := tmux.AttachSpec{Name: parsed.Target, Cwd: parsed.Cwd}
			if err := dispatchAttach(ctx, tx, c, spec); err != nil {
				fmt.Fprintf(os.Stderr, "tmux-login: attach: %v\n", err)
			}
			return nil

		case r.IsTypeToCreate():
			// Type-to-create: user typed a name fzf didn't match, hit Enter.
			// We always run our dir picker (regardless of sesh availability)
			// because the user picked an explicit name that needs an explicit
			// dir; sesh's name→cwd resolution would default to $HOME, which
			// isn't what we want here. After dir is chosen, attach via the
			// existing tmux path that takes -c <DIR>.
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

		// No actionable result; treat as cancel.
		return nil
	}
}

// buildSessionItems returns the items to feed the picker. With sesh
// available we get its multi-source list (tmux + zoxide + sesh.toml +
// tmuxinator) with Nerd Font icons; otherwise we fall back to the
// internal sources.Sessions list (tmux only).
func buildSessionItems(ctx context.Context, useSesh bool, sx *sesh.Client, tx *tmux.Client, c *cache.Cache) ([]sources.Item, error) {
	if useSesh {
		seshItems, err := sx.List(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]sources.Item, 0, len(seshItems))
		for _, si := range seshItems {
			out = append(out, sources.Item{
				Mode:       sources.ModeSessions,
				Display:    si.Display,
				ActionKind: sources.ActionAttach,
				Target:     si.Target,
				// Cwd left empty; sesh resolves it from name.
			})
		}
		return out, nil
	}
	return (&sources.Sessions{Tmux: tx, Cache: c}).Items(ctx)
}

// RunLast jumps to the last-attached session. Wired to `tmux-login last`
// (cmd dispatcher) and to the M-L keybinding in share/tmux.conf.
// When sesh isn't available, prints a diagnostic to stderr — there's no
// equivalent in our internal code path (tmux's own buffer doesn't track
// session attach order without server hooks).
func RunLast(ctx context.Context) error {
	sx := sesh.New()
	if !sx.Available() {
		fmt.Fprintln(os.Stderr, "tmux-login: `last` requires sesh — install via `brew install sesh`")
		return nil
	}
	if err := sx.Last(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "tmux-login: sesh last: %v\n", err)
	}
	return nil
}

func dispatchAttach(ctx context.Context, tx *tmux.Client, c *cache.Cache, spec tmux.AttachSpec) error {
	if err := tx.Attach(ctx, spec); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	_ = c.RecordAttach(spec.Name)
	if spec.Cwd != "" {
		_ = c.RecordCwd(spec.Name, spec.Cwd)
		_ = c.RecordRecentDir(spec.Cwd)
	}
	return nil
}
