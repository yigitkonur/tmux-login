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

	// Loop so ctrl-x can kill the highlighted session and re-render the
	// list. Every other terminal action (attach, type-to-create, skip,
	// cancel) returns immediately.
	for {
		sessions, err := (&sources.Sessions{Tmux: tx, Cache: c}).Items(ctx)
		if err != nil {
			// Soft-fail: present an empty list rather than erroring out the login.
			fmt.Fprintf(os.Stderr, "tmux-login: list-sessions: %v\n", err)
		}
		tr.Mark("list_done", "n=", len(sessions))

		// Phantom-cache GC.
		liveNames := make([]string, 0, len(sessions))
		for _, s := range sessions {
			liveNames = append(liveNames, s.Target)
		}
		c.GC(liveNames, err == nil)

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

		// Ctrl-X: kill the highlighted session, then re-render. No-op if
		// the cursor is on the [skip] sentinel or no row is selected.
		if r.Key == "ctrl-x" {
			if parsed.OK && parsed.Action == "attach" {
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
			spec := tmux.AttachSpec{Name: parsed.Target, Cwd: parsed.Cwd}
			if err := dispatchAttach(ctx, tx, c, spec); err != nil {
				fmt.Fprintf(os.Stderr, "tmux-login: attach: %v\n", err)
			}
			return nil

		case r.IsTypeToCreate():
			// Second fzf round: pick a starting directory.
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
