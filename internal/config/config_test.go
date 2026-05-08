package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMUX_LOGIN_ROOTS", "")
	t.Setenv("TMUX_LOGIN_PRUNE", "")
	t.Setenv("TMUX_LOGIN_SKIP", "")
	t.Setenv("TMUX_LOGIN_PERF", "")

	cfg := Load()
	if !strings.HasPrefix(cfg.CacheDir, tmp) {
		t.Errorf("cache dir = %s; want under %s", cfg.CacheDir, tmp)
	}
	if !strings.HasSuffix(cfg.CacheDir, "tmux-login") {
		t.Errorf("cache dir = %s; want suffix tmux-login", cfg.CacheDir)
	}
	if cfg.Skip {
		t.Error("skip should default false")
	}
	if cfg.Perf {
		t.Error("perf should default false")
	}
	if len(cfg.Roots) == 0 {
		t.Error("roots should fall back to home")
	}
	if cfg.Roots[0] != tmp {
		t.Errorf("with no extant project dirs, root should be home; got %s", cfg.Roots[0])
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "altcache"))
	t.Setenv("TMUX_LOGIN_ROOTS", "/a:/b:/c")
	t.Setenv("TMUX_LOGIN_PRUNE", "node_modules:foo:bar")
	t.Setenv("TMUX_LOGIN_SKIP", "1")
	t.Setenv("TMUX_LOGIN_PERF", "1")

	cfg := Load()
	if cfg.CacheDir != filepath.Join(tmp, "altcache", "tmux-login") {
		t.Errorf("cache dir = %s", cfg.CacheDir)
	}
	if len(cfg.Roots) != 3 || cfg.Roots[0] != "/a" {
		t.Errorf("roots = %v", cfg.Roots)
	}
	if len(cfg.PruneExtras) != 3 {
		t.Errorf("prune extras = %v", cfg.PruneExtras)
	}
	if !cfg.Skip || !cfg.Perf {
		t.Error("skip/perf should be true")
	}
}

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "c"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "s"))
	cfg := Load()
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	for _, d := range []string{cfg.CacheDir, cfg.StateDir} {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			t.Errorf("dir %s not created (err=%v)", d, err)
		}
	}
}
