// Package cache owns the on-disk state under $XDG_CACHE_HOME/tmux-login:
//
//	attached/<name>     zero-byte file; mtime = last attach timestamp
//	cwds/<name>         text file; one line = the start cwd we attached at
//	recent_dirs         MRU list of recently-attached dirs, max 50
//	.sessions.txt       single-snapshot tmux list-sessions output (shared
//	                    with the picker preview pane to avoid forking tmux
//	                    on every keystroke)
//
// All writes are atomic via temp+rename so concurrent SSH logins can't
// corrupt the destination. Directly mirrors zellij-login's cache layout.
package cache

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxRecent = 50
)

type Cache struct {
	Root string // $XDG_CACHE_HOME/tmux-login
}

func New(root string) *Cache {
	return &Cache{Root: root}
}

func (c *Cache) ensureSubdir(rel string) (string, error) {
	d := filepath.Join(c.Root, rel)
	return d, os.MkdirAll(d, 0o755)
}

// RecordAttach touches $cache/attached/<name> so subsequent picker runs sort
// it by recency (most-recent attach first).
func (c *Cache) RecordAttach(name string) error {
	d, err := c.ensureSubdir("attached")
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(d, name))
	if err != nil {
		return err
	}
	return f.Close()
}

// RecordCwd writes the cwd a session was attached at, for preview pane use.
func (c *Cache) RecordCwd(name, cwd string) error {
	d, err := c.ensureSubdir("cwds")
	if err != nil {
		return err
	}
	tmp := filepath.Join(d, ".cwd."+name+".tmp")
	if err := os.WriteFile(tmp, []byte(cwd+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(d, name))
}

// RecordRecentDir prepends path to recent_dirs, dedup'd, capped at 50.
func (c *Cache) RecordRecentDir(path string) error {
	if err := os.MkdirAll(c.Root, 0o755); err != nil {
		return err
	}
	file := filepath.Join(c.Root, "recent_dirs")
	existing, _ := os.ReadFile(file) // missing → empty

	var out bytes.Buffer
	out.WriteString(path)
	out.WriteByte('\n')

	count := 1
	sc := bufio.NewScanner(bytes.NewReader(existing))
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line == path {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
		count++
		if count >= maxRecent {
			break
		}
	}

	tmp := fmt.Sprintf("%s.tmp.%d", file, os.Getpid())
	if err := os.WriteFile(tmp, out.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// RecentDirs returns the MRU list, filtered to existing directories.
func (c *Cache) RecentDirs() []string {
	data, err := os.ReadFile(filepath.Join(c.Root, "recent_dirs"))
	if err != nil {
		return nil
	}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		if st, err := os.Stat(line); err == nil && st.IsDir() {
			out = append(out, line)
		}
	}
	return out
}

// AttachedAt returns the unix-seconds mtime of the attached marker, or 0 if
// no marker exists. Used by the sessions source to sort by recency.
func (c *Cache) AttachedAt(name string) int64 {
	st, err := os.Stat(filepath.Join(c.Root, "attached", name))
	if err != nil {
		return 0
	}
	return st.ModTime().Unix()
}

// CwdOf returns the recorded cwd for a session, or "" if unknown.
func (c *Cache) CwdOf(name string) string {
	data, err := os.ReadFile(filepath.Join(c.Root, "cwds", name))
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
}

// DropSession removes attached/<name> and cwds/<name>. Best-effort.
func (c *Cache) DropSession(name string) {
	_ = os.Remove(filepath.Join(c.Root, "attached", name))
	_ = os.Remove(filepath.Join(c.Root, "cwds", name))
}

// SessionsSnapshotPath is where snapshot.go writes the tab-delimited live
// list. Public so callers can read directly without going through us.
func (c *Cache) SessionsSnapshotPath() string {
	return filepath.Join(c.Root, ".sessions.txt")
}

// WriteSessionsSnapshot writes data to .sessions.txt atomically (temp+rename)
// so concurrent writers can't corrupt readers.
func (c *Cache) WriteSessionsSnapshot(data []byte) error {
	if err := os.MkdirAll(c.Root, 0o755); err != nil {
		return err
	}
	dest := c.SessionsSnapshotPath()
	tmp := fmt.Sprintf("%s.tmp.%d", dest, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// ReadSessionsSnapshot returns the snapshot bytes, or nil if absent.
func (c *Cache) ReadSessionsSnapshot() []byte {
	data, _ := os.ReadFile(c.SessionsSnapshotPath())
	return data
}

// GC sweeps stale attached/<name> and cwds/<name> entries whose session no
// longer exists in liveNames. liveNames is the authoritative live set; an
// empty slice with hasLive=true means "tmux is up but has no sessions" and
// we should clean every cache entry.
func (c *Cache) GC(liveNames []string, hasLive bool) {
	if !hasLive {
		// tmux is unreachable / not running. Don't touch the cache — we'd
		// otherwise nuke valid markers on every login when the daemon is
		// briefly down.
		return
	}
	live := make(map[string]struct{}, len(liveNames))
	for _, n := range liveNames {
		live[n] = struct{}{}
	}
	for _, sub := range []string{"attached", "cwds"} {
		dir := filepath.Join(c.Root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if _, ok := live[e.Name()]; !ok {
				_ = os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
}
