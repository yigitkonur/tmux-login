package sources

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/yigitkonur/tmux-login/internal/cache"
)

type Recent struct {
	Cache *cache.Cache
	Home  string
}

func (r *Recent) Mode() Mode { return ModeRecent }

func (r *Recent) Items(ctx context.Context) ([]Item, error) {
	dirs := r.Cache.RecentDirs()
	out := make([]Item, 0, len(dirs))
	for _, d := range dirs {
		name := deriveSessionName(filepath.Base(d))
		disp := d
		if r.Home != "" && strings.HasPrefix(d, r.Home) {
			disp = "~" + strings.TrimPrefix(d, r.Home)
		}
		out = append(out, Item{
			Mode:       ModeRecent,
			Display:    name + "\t" + disp,
			ActionKind: ActionAttach,
			Target:     name,
			Cwd:        d,
		})
	}
	return out, nil
}
