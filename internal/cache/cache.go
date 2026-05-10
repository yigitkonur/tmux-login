// Package cache owns the on-disk MRU list of recently-attached project
// directories and per-session attach metadata at $XDG_CACHE_HOME/tmux-login.
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
	"strings"
)

const maxRecent = 50

type Cache struct {
	Root string // $XDG_CACHE_HOME/tmux-login
}

func New(root string) *Cache {
	return &Cache{Root: root}
}

func safeName(name string) (string, bool) {
	if name == "" || name == "." || name == ".." {
		return "", false
	}
	if strings.ContainsAny(name, `/`+"\x00") {
		return "", false
	}
	return name, true
}

// RecordAttach updates the session's last-attached marker. Call before attach
// paths that may exec so successful outside-tmux attaches still persist recency.
func (c *Cache) RecordAttach(name string) error {
	name, ok := safeName(name)
	if !ok {
		return nil
	}
	dir := filepath.Join(c.Root, "attached")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file := filepath.Join(dir, name)
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

// RecordCwd stores the starting directory for sessions this tool creates.
func (c *Cache) RecordCwd(name, cwd string) error {
	name, ok := safeName(name)
	if !ok || cwd == "" {
		return nil
	}
	dir := filepath.Join(c.Root, "cwds")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(cwd+"\n"), 0o644)
}

// DropSession removes best-effort metadata for a killed session.
func (c *Cache) DropSession(name string) {
	name, ok := safeName(name)
	if !ok {
		return
	}
	_ = os.Remove(filepath.Join(c.Root, "attached", name))
	_ = os.Remove(filepath.Join(c.Root, "cwds", name))
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
