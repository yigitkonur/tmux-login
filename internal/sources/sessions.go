package sources

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/yigitkonur/tmux-login/internal/cache"
	"github.com/yigitkonur/tmux-login/internal/tmux"
)

type Sessions struct {
	Tmux  *tmux.Client
	Cache *cache.Cache
}

func (s *Sessions) Mode() Mode { return ModeSessions }

func (s *Sessions) Items(ctx context.Context) ([]Item, error) {
	live, err := s.Tmux.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	type pair struct {
		sess  tmux.Session
		mtime int64
	}
	pairs := make([]pair, 0, len(live))
	for _, sess := range live {
		mtime := s.Cache.AttachedAt(sess.Name)
		// Fall back to tmux's own session_last_attached so brand-new sessions
		// (no attached/<name> marker yet) still sort sensibly.
		if mtime == 0 {
			mtime = sess.LastAttached
		}
		pairs = append(pairs, pair{sess, mtime})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].mtime != pairs[j].mtime {
			return pairs[i].mtime > pairs[j].mtime
		}
		return pairs[i].sess.Name < pairs[j].sess.Name
	})

	out := make([]Item, 0, len(pairs))
	now := time.Now().Unix()
	for _, p := range pairs {
		out = append(out, Item{
			Mode:       ModeSessions,
			Display:    formatSessionLine(p.sess, p.mtime, now),
			ActionKind: ActionAttach,
			Target:     p.sess.Name,
			Cwd:        p.sess.Path,
		})
	}
	return out, nil
}

func formatSessionLine(s tmux.Session, mtime, now int64) string {
	icon := "○"
	if s.Attached {
		icon = "●"
	}
	last := "never"
	if mtime > 0 {
		last = humanAgo(now - mtime)
	}
	return fmt.Sprintf("%s\t%s\t%s", icon, s.Name, last)
}

// humanAgo renders a duration with mixed precision so transitions across
// thresholds don't lose information. Examples:
//
//	30s   → "just now"
//	5m    → "5m ago"
//	1h13m → "1h13m ago"   (kept precise; was just "1h ago" before)
//	1h    → "1h ago"      (drop minutes when zero, no "1h0m")
//	2d5h  → "2d5h ago"
//	7d+   → "7d ago"      (no point keeping hour-precision for week-old)
func humanAgo(seconds int64) string {
	switch {
	case seconds < 60:
		return "just now"
	case seconds < 3600:
		return fmt.Sprintf("%dm ago", seconds/60)
	case seconds < 86400:
		h := seconds / 3600
		m := (seconds % 3600) / 60
		if m == 0 {
			return fmt.Sprintf("%dh ago", h)
		}
		return fmt.Sprintf("%dh%dm ago", h, m)
	case seconds < 7*86400:
		d := seconds / 86400
		h := (seconds % 86400) / 3600
		if h == 0 {
			return fmt.Sprintf("%dd ago", d)
		}
		return fmt.Sprintf("%dd%dh ago", d, h)
	default:
		return fmt.Sprintf("%dd ago", seconds/86400)
	}
}
