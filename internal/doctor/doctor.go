// Package doctor renders the `tmux-login doctor` diagnostic. Output is
// key=value lines for grep-friendliness.
package doctor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yigitkonur/tmux-login/internal/config"
	"github.com/yigitkonur/tmux-login/internal/install"
	"github.com/yigitkonur/tmux-login/internal/tmux"
	"github.com/yigitkonur/tmux-login/internal/version"
)

func Run(ctx context.Context, w io.Writer) error {
	cfg := config.Load()
	tx := tmux.New()

	fmt.Fprintf(w, "tmux-login: %s (commit %s built %s)\n", version.Version, version.Commit, version.Date)
	fmt.Fprintf(w, "go: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	if v := tx.Version(ctx); v != "" {
		path, _ := exec.LookPath("tmux")
		fmt.Fprintf(w, "tmux: %s (%s)\n", v, path)
	} else {
		fmt.Fprintln(w, "tmux: (not on PATH)")
	}

	if path, err := exec.LookPath("fzf"); err == nil {
		out, _ := exec.CommandContext(ctx, "fzf", "--version").Output()
		ver := strings.TrimSpace(string(out))
		fmt.Fprintf(w, "fzf: %s (%s)\n", ver, path)
	} else {
		fmt.Fprintln(w, "fzf: (not on PATH)")
	}

	if tmux.InsideTmux() {
		fmt.Fprintf(w, "inside-tmux: yes ($TMUX=%s)\n", os.Getenv("TMUX"))
	} else {
		fmt.Fprintln(w, "inside-tmux: no")
	}

	clip, _ := tx.Output(ctx, "show", "-gv", "set-clipboard")
	if c := strings.TrimSpace(string(clip)); c != "" {
		fmt.Fprintf(w, "set-clipboard: %s\n", c)
	} else {
		fmt.Fprintln(w, "set-clipboard: (n/a; tmux server not running)")
	}

	fmt.Fprintf(w, "default-shell: %s\n", os.Getenv("SHELL"))
	fmt.Fprintf(w, "TERM: %s\n", os.Getenv("TERM"))
	if tp := os.Getenv("TERM_PROGRAM"); tp != "" {
		fmt.Fprintf(w, "terminal: %s\n", tp)
	}

	fmt.Fprintf(w, "xdg-cache-home: %s\n", filepath.Dir(cfg.CacheDir))
	fmt.Fprintf(w, "xdg-data-home: %s\n", filepath.Dir(cfg.DataDir))
	fmt.Fprintf(w, "cache-dir: %s\n", cfg.CacheDir)

	roots := strings.Join(cfg.Roots, ":")
	fmt.Fprintf(w, "config-roots: %s\n", roots)

	tmuxConf := filepath.Join(cfg.Home, install.TmuxConfBasename)
	zshrc := filepath.Join(cfg.Home, install.ZshrcBasename)
	fmt.Fprintf(w, "marker-tmux-conf: %s\n", markerStatus(tmuxConf))
	fmt.Fprintf(w, "marker-zshrc: %s\n", markerStatus(zshrc))

	return nil
}

func markerStatus(path string) string {
	had, err := install.HasMarker(path)
	if err != nil {
		return "error: " + err.Error()
	}
	if had {
		return "present"
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "absent (file does not exist)"
	}
	return "absent"
}
