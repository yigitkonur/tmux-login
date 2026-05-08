package sources

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yigitkonur/tmux-login/internal/cache"
)

// defaultPrune mirrors zellij-login's verbatim list, plus a few common build
// dirs that consistently noise up project pickers.
var defaultPrune = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	".cache":       {},
	"Library":      {},
	".Trash":       {},
	".cargo":       {},
	".rustup":      {},
	".npm":         {},
	"dist":         {},
	"build":        {},
	"target":       {},
	"vendor":       {},
	"__pycache__":  {},
	".next":        {},
	".nuxt":        {},
	".venv":        {},
	"venv":         {},
	".tox":         {},
}

type Projects struct {
	Roots       []string // absolute paths
	PruneExtras []string // extra basenames the user supplied via TMUX_LOGIN_PRUNE
	Cache       *cache.Cache
	Home        string
}

func (p *Projects) Mode() Mode { return ModeProjects }

func (p *Projects) Items(ctx context.Context) ([]Item, error) {
	prune := make(map[string]struct{}, len(defaultPrune)+len(p.PruneExtras))
	for k, v := range defaultPrune {
		prune[k] = v
	}
	for _, k := range p.PruneExtras {
		prune[k] = struct{}{}
	}

	seen := make(map[string]struct{})
	var dirs []string

	add := func(d string) {
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		dirs = append(dirs, d)
	}

	// MRU first — entries the user has already attached to bubble to the top.
	for _, d := range p.Cache.RecentDirs() {
		add(d)
	}

	for _, root := range p.Roots {
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			continue
		}
		// Depth-0: the root itself.
		add(root)
		// Depth-1: immediate children, with prune. Sorted by mtime descending
		// so projects you've actually touched bubble up; ties broken by name
		// asc so the output is stable across boring days.
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		type kid struct {
			name  string
			mtime int64
		}
		kids := make([]kid, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, blocked := prune[e.Name()]; blocked {
				continue
			}
			var mt int64
			if info, err := e.Info(); err == nil {
				mt = info.ModTime().Unix()
			}
			kids = append(kids, kid{name: e.Name(), mtime: mt})
		}
		sort.SliceStable(kids, func(i, j int) bool {
			if kids[i].mtime != kids[j].mtime {
				return kids[i].mtime > kids[j].mtime
			}
			return kids[i].name < kids[j].name
		})
		for _, k := range kids {
			add(filepath.Join(root, k.name))
		}
	}

	out := make([]Item, 0, len(dirs))
	for _, d := range dirs {
		name := deriveSessionName(filepath.Base(d))
		out = append(out, Item{
			Mode:       ModeProjects,
			Display:    formatProjectLine(d, p.Home),
			ActionKind: ActionAttach,
			Target:     name,
			Cwd:        d,
		})
	}
	return out, nil
}

func deriveSessionName(base string) string {
	// tmux session names can't contain `.` (interpreted as session.window).
	return strings.ReplaceAll(base, ".", "_")
}

// formatProjectLine returns just the tilde-collapsed path. fzf's fuzzy
// match still picks up the basename inside the path string, so we save
// a column without losing search effectiveness — and we no longer show
// the basename twice (once as a column, once inside the path).
func formatProjectLine(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
