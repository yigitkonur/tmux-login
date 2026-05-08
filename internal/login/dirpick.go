package login

import (
	"context"
	"os"
	"path/filepath"

	"github.com/yigitkonur/tmux-login/internal/cache"
	"github.com/yigitkonur/tmux-login/internal/config"
	"github.com/yigitkonur/tmux-login/internal/picker"
	"github.com/yigitkonur/tmux-login/internal/sources"
)

// pickDirectory runs the second-round fzf when the user typed a new session
// name and now needs to choose a starting directory.
//
// Candidates from the projects source (MRU first, then roots+depth-1
// children sorted by mtime desc), with $HOME demoted to the bottom — most
// users live inside one of the configured roots, not at $HOME, so making
// the first root the cursor default cuts ten keystrokes a day.
//
// The query is pre-filled with the typed session name so a project with a
// matching basename (e.g. "testo" → ~/dev/testo) lands under the cursor
// immediately. If the query has zero matches and the user presses Enter,
// we mkdir the path under the first root and use that — the fast path for
// "I'm starting a new project named X."
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
	for _, it := range items {
		// Override target so it's the new session's name (the projects source
		// derived a target from basename; here we attach the *typed* name).
		it.Target = sessionName
		lines = append(lines, picker.Encode(it))
	}
	// $HOME at the bottom: still pickable when the user actually wants it,
	// out of the way otherwise.
	if cfg.Home != "" {
		lines = append(lines, picker.Encode(sources.Item{
			Mode:       sources.ModeProjects,
			ActionKind: sources.ActionAttach,
			Target:     sessionName,
			Cwd:        cfg.Home,
			Display:    "~",
		}))
	}

	r, err := picker.Pick(ctx, picker.Spec{
		Prompt:     "dir for '" + sessionName + "' > ",
		Header:     picker.HeaderFor("new", "enter=use/create dir  C-u=clear filter  esc=cancel"),
		Lines:      lines,
		Query:      sessionName,
		PrintQuery: true,
	}, nil)
	if err != nil || r.IsCancelled() {
		return "", err
	}

	// Type-to-create at the directory level: if the user's query matched no
	// existing dir and they hit Enter, mkdir <first-root>/<query> and use
	// it as the start dir. Picks <first-root> rather than $HOME because
	// users who configured roots want new projects under those, not
	// scattered at home.
	if r.IsTypeToCreate() {
		base := cfg.Home
		if len(cfg.Roots) > 0 {
			base = cfg.Roots[0]
		}
		target := filepath.Join(base, r.Query)
		if err := os.MkdirAll(target, 0o755); err != nil {
			return "", err
		}
		return target, nil
	}

	if !r.Parsed().OK {
		return "", nil
	}
	return r.Parsed().Cwd, nil
}
