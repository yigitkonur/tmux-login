// Package cache owns the on-disk MRU list of recently-attached project
// directories at $XDG_CACHE_HOME/tmux-login/recent_dirs. With sesh as the
// session engine, sesh + zoxide handle session-state caching; we only
// keep the recent-dirs file because it feeds the type-to-create dir
// picker, which sesh has no equivalent for.
//
// Writes are atomic via temp+rename so concurrent SSH logins can't
// corrupt the destination.
package cache

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

const maxRecent = 50

type Cache struct {
	Root string // $XDG_CACHE_HOME/tmux-login
}

func New(root string) *Cache {
	return &Cache{Root: root}
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
