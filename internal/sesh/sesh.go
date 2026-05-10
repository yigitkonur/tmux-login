// Package sesh wraps the joshmedeski/sesh CLI when it's on PATH. We use sesh
// as the session-list source ("sesh list --json") and as the idempotent
// attach engine ("sesh connect NAME"). When sesh isn't available, the login
// flow falls back to internal/sources/sessions + internal/tmux/attach.
//
// Design mirrors internal/tmux/client.go: the Client wraps os/exec, inherits
// os.Environ() unmodified (NEVER force LANG=C — that bug bit us hard in
// commit af47aa2 by mangling tmux UTF-8 server-wide).
package sesh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

type Client struct {
	bin       string
	available *bool
	availOnce sync.Once
}

// New returns a sesh client. SESH_BIN env var overrides PATH lookup
// (mirrors TMUX_BIN — the runtime test stubs use this hook). When SESH_BIN
// is unset and sesh is not on PATH, Available() returns false and the
// caller should fall back to the no-sesh path.
func New() *Client {
	bin := os.Getenv("SESH_BIN")
	if bin == "" {
		bin = "sesh"
	}
	return &Client{bin: bin}
}

// Available reports whether sesh is callable. Result is cached for the
// lifetime of the Client so the LookPath cost (~500 µs) isn't paid per
// call. The login flow only constructs one Client per Run().
func (c *Client) Available() bool {
	c.availOnce.Do(func() {
		// If SESH_BIN points at an explicit path (set by tests or by users
		// who installed under a non-PATH prefix), honor it without LookPath.
		if _, err := os.Stat(c.bin); err == nil {
			t := true
			c.available = &t
			return
		}
		_, err := exec.LookPath(c.bin)
		t := err == nil
		c.available = &t
	})
	return c.available != nil && *c.available
}

// Item is one row from `sesh list --icons`. Display is the raw line,
// ANSI-coloured and icon-prefixed; pass it straight to fzf with --ansi.
// Target is the bare name/path that `sesh connect` accepts.
type Item struct {
	Display  string
	Target   string
	Path     string
	Source   string
	Rank     int64
	Attached int
}

// List runs `sesh list --json` and parses the output into Items. Older test
// stubs and sesh variants that emit icon lines still work through parseList.
func (c *Client) List(ctx context.Context) ([]Item, error) {
	cmd := exec.CommandContext(ctx, c.bin, "list", "--json")
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sesh list: %w (stderr=%s)", err, bytes.TrimSpace(stderr.Bytes()))
	}
	if items, ok := parseJSONList(stdout.Bytes()); ok {
		sortItems(items)
		return items, nil
	}
	return parseList(stdout.Bytes()), nil
}

type jsonItem struct {
	Src      string
	Name     string
	Path     string
	Attached int
	Windows  int
}

func parseJSONList(b []byte) ([]Item, bool) {
	var rows []jsonItem
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, false
	}
	out := make([]Item, 0, len(rows))
	for _, row := range rows {
		target := row.Name
		if target == "" {
			target = row.Path
		}
		if target == "" {
			continue
		}
		out = append(out, Item{
			Display:  displayJSONItem(row),
			Target:   target,
			Path:     row.Path,
			Source:   row.Src,
			Rank:     itemRank(row.Src, row.Path, target),
			Attached: row.Attached,
		})
	}
	return out, true
}

func itemRank(src, path, target string) int64 {
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		return st.ModTime().Unix()
	}
	if src == "zoxide" {
		if expanded := expandTilde(target); expanded != target {
			if st, err := os.Stat(expanded); err == nil && st.IsDir() {
				return st.ModTime().Unix()
			}
		}
	}
	return 0
}

func sortItems(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		ci, cj := itemClass(items[i]), itemClass(items[j])
		if ci != cj {
			return ci < cj
		}
		if items[i].Rank != items[j].Rank {
			return items[i].Rank > items[j].Rank
		}
		return strings.ToLower(items[i].Target) < strings.ToLower(items[j].Target)
	})
}

func itemClass(it Item) int {
	if it.Source == "tmux" {
		if it.Attached > 0 {
			return 0
		}
		return 1
	}
	path := it.Path
	if path == "" {
		path = expandTilde(it.Target)
	}
	if isDevChild(path) {
		return 2
	}
	if isBroadRoot(path) {
		return 4
	}
	if isExistingDir(path) {
		return 3
	}
	return 5
}

func isDevChild(path string) bool {
	home := os.Getenv("HOME")
	if home == "" || path == "" {
		return false
	}
	dev := filepath.Join(home, "dev")
	rel, err := filepath.Rel(dev, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return false
	}
	return !strings.Contains(rel, string(filepath.Separator))
}

func isExistingDir(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func isBroadRoot(path string) bool {
	home := os.Getenv("HOME")
	if home == "" {
		return false
	}
	switch path {
	case home, filepath.Join(home, "dev"), filepath.Join(home, "code"), filepath.Join(home, "projects"), filepath.Join(home, "work"):
		return true
	default:
		return false
	}
}

func expandTilde(path string) string {
	home := os.Getenv("HOME")
	if home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}

func displayJSONItem(row jsonItem) string {
	target := row.Name
	if target == "" {
		target = row.Path
	}
	switch row.Src {
	case "tmux":
		if row.Attached > 0 {
			return "● " + target
		}
		return "○ " + target
	case "zoxide":
		return " " + target
	default:
		return "◇ " + target
	}
}

// Connect runs `sesh connect NAME`. When called from outside tmux, sesh
// itself execs into `tmux attach`; we therefore syscall.Exec into sesh so
// our Go process is replaced (the same shape as internal/tmux.execAttach).
// When inside tmux, sesh switches the calling client; that's a quick
// roundtrip and we just call it as a subprocess.
func (c *Client) Connect(ctx context.Context, name string) error {
	if name == "" {
		return os.ErrInvalid
	}
	if os.Getenv("TMUX") != "" {
		// Inside tmux: sesh handles switch-client; subprocess is fine.
		cmd := exec.CommandContext(ctx, c.bin, "connect", name)
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// Outside tmux: replace ourselves with sesh, which in turn execs tmux.
	bin, err := exec.LookPath(c.bin)
	if err != nil {
		// SESH_BIN points at an absolute path (e.g. test stub). Fall
		// through and exec by full path.
		if _, statErr := os.Stat(c.bin); statErr == nil {
			bin = c.bin
		} else {
			return err
		}
	}
	return syscall.Exec(bin, []string{c.bin, "connect", name}, os.Environ())
}

// Last runs `sesh last`. Same exec semantics as Connect — we always replace
// ourselves with sesh when outside tmux.
func (c *Client) Last(ctx context.Context) error {
	if os.Getenv("TMUX") != "" {
		cmd := exec.CommandContext(ctx, c.bin, "last")
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	bin, err := exec.LookPath(c.bin)
	if err != nil {
		if _, statErr := os.Stat(c.bin); statErr == nil {
			bin = c.bin
		} else {
			return err
		}
	}
	return syscall.Exec(bin, []string{c.bin, "last"}, os.Environ())
}
