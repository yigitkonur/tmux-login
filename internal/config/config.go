// Package config resolves XDG paths and env-var-driven knobs once at startup.
// Defaults are picked so the binary works with no env at all.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Home        string
	CacheDir    string // $XDG_CACHE_HOME/tmux-login
	StateDir    string // $XDG_STATE_HOME/tmux-login
	DataDir     string // $XDG_DATA_HOME/tmux-login
	ConfigDir   string // $XDG_CONFIG_HOME/tmux-login
	RuntimeDir  string // $XDG_RUNTIME_DIR (or fallback to TMPDIR)
	Roots       []string
	PruneExtras []string
	Skip        bool
	Perf        bool
}

func defaultRoots(home string) []string {
	candidates := []string{"dev", "code", "projects", "work", "Developer", "src", "research"}
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		p := filepath.Join(home, c)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = append(out, home)
	}
	return out
}

func xdg(envVar, fallbackRel, home string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return filepath.Join(home, fallbackRel)
}

// Load reads env vars and computes the config. Never errors — falls back to
// reasonable defaults for missing values. Callers may mutate the returned
// struct, but typical code reads once at startup.
func Load() *Config {
	home := os.Getenv("HOME")
	if home == "" {
		// Worst-case fallback. tmux's own behavior here is to refuse, but we'd
		// rather degrade than crash on misconfigured environments.
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}

	cfg := &Config{
		Home:       home,
		CacheDir:   filepath.Join(xdg("XDG_CACHE_HOME", ".cache", home), "tmux-login"),
		StateDir:   filepath.Join(xdg("XDG_STATE_HOME", ".local/state", home), "tmux-login"),
		DataDir:    filepath.Join(xdg("XDG_DATA_HOME", ".local/share", home), "tmux-login"),
		ConfigDir:  filepath.Join(xdg("XDG_CONFIG_HOME", ".config", home), "tmux-login"),
		RuntimeDir: os.Getenv("XDG_RUNTIME_DIR"),
	}
	if cfg.RuntimeDir == "" {
		cfg.RuntimeDir = os.TempDir()
	}

	if v := os.Getenv("TMUX_LOGIN_ROOTS"); v != "" {
		for _, p := range strings.Split(v, ":") {
			if p != "" {
				cfg.Roots = append(cfg.Roots, p)
			}
		}
	}
	if len(cfg.Roots) == 0 {
		cfg.Roots = defaultRoots(home)
	}

	if v := os.Getenv("TMUX_LOGIN_PRUNE"); v != "" {
		for _, p := range strings.Split(v, ":") {
			if p != "" {
				cfg.PruneExtras = append(cfg.PruneExtras, p)
			}
		}
	}

	cfg.Skip = os.Getenv("TMUX_LOGIN_SKIP") == "1"

	if os.Getenv("TMUX_LOGIN_PERF") == "1" {
		cfg.Perf = true
	} else if _, err := os.Stat(filepath.Join(cfg.StateDir, "perf.on")); err == nil {
		cfg.Perf = true
	}

	return cfg
}

// EnsureDirs creates cache/state dirs if missing. Callers invoke before
// touching the cache; safe to call repeatedly.
func (c *Config) EnsureDirs() error {
	for _, d := range []string{c.CacheDir, c.StateDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
