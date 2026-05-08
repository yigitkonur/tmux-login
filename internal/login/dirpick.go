package login

import (
	"context"

	"github.com/yigitkonur/tmux-login/internal/cache"
	"github.com/yigitkonur/tmux-login/internal/config"
	"github.com/yigitkonur/tmux-login/internal/picker"
	"github.com/yigitkonur/tmux-login/internal/sources"
)

// pickDirectory runs the second-round fzf when the user typed a new session
// name and now needs to choose a starting directory.
//
// Candidates: $HOME first, MRU recents, configured roots and their depth-1
// children — exact mirror of zellij-login's _zl_dir_candidates ordering
// (zellij-ssh-login.zsh:185-202).
func pickDirectory(ctx context.Context, cfg *config.Config, c *cache.Cache, sessionName string) (string, error) {
	proj := &sources.Projects{
		Roots:       cfg.Roots,
		PruneExtras: cfg.PruneExtras,
		Cache:       c,
		Home:        cfg.Home,
	}
	items, err := proj.Items(ctx)
	if err != nil {
		return "", err
	}

	lines := make([]string, 0, len(items)+1)
	// Pin $HOME at the top so Enter on an empty query lands you at home.
	if cfg.Home != "" {
		lines = append(lines, picker.Encode(sources.Item{
			Mode:       sources.ModeProjects,
			ActionKind: sources.ActionAttach,
			Target:     sessionName,
			Cwd:        cfg.Home,
			Display:    "~",
		}))
	}
	for _, it := range items {
		// Override target so it's the new session's name (basename-derived was
		// for projects mode; here we attach the *typed* name to the chosen dir).
		it.Target = sessionName
		lines = append(lines, picker.Encode(it))
	}

	r, err := picker.Pick(ctx, picker.Spec{
		Prompt:     "dir for '" + sessionName + "' > ",
		Header:     picker.HeaderFor("new", "enter=use dir  esc=cancel"),
		Lines:      lines,
		PrintQuery: true,
	}, nil)
	if err != nil || r.IsCancelled() {
		return "", err
	}
	if !r.Parsed().OK {
		return "", nil
	}
	return r.Parsed().Cwd, nil
}
