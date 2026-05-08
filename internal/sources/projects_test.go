package sources

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yigitkonur/tmux-login/internal/cache"
)

// TestProjectsMtimeSort verifies depth-1 children come back in mtime-desc
// order with name-asc tiebreaks.
func TestProjectsMtimeSort(t *testing.T) {
	root := t.TempDir()
	// Create three children. Stagger mtimes via time.Sleep between MkdirAll
	// calls (1.1s — enough to clear filesystem mtime granularity).
	mk := func(name string) {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mk("aardvark") // oldest
	time.Sleep(1100 * time.Millisecond)
	mk("zebra")
	time.Sleep(1100 * time.Millisecond)
	mk("middle") // newest

	p := &Projects{
		Roots: []string{root},
		Cache: cache.New(t.TempDir()),
		Home:  os.Getenv("HOME"),
	}
	items, err := p.Items(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// First item is the root itself; remaining are children in mtime-desc order.
	if len(items) < 4 {
		t.Fatalf("expected ≥4 items (root + 3 kids), got %d: %+v", len(items), items)
	}
	if !strings.HasSuffix(items[0].Cwd, root) && items[0].Cwd != root {
		t.Errorf("items[0].Cwd = %s; want root %s", items[0].Cwd, root)
	}
	wantOrder := []string{"middle", "zebra", "aardvark"}
	for i, want := range wantOrder {
		got := filepath.Base(items[i+1].Cwd)
		if got != want {
			t.Errorf("items[%d] = %s; want %s (mtime sort)", i+1, got, want)
		}
	}
}

// TestProjectsMtimeTiesByName: children with identical mtimes sort
// alphabetically as a stable tiebreak.
func TestProjectsMtimeTiesByName(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Force identical mtime on all three.
	now := time.Now()
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		if err := os.Chtimes(filepath.Join(root, name), now, now); err != nil {
			t.Fatal(err)
		}
	}

	p := &Projects{Roots: []string{root}, Cache: cache.New(t.TempDir())}
	items, _ := p.Items(context.Background())
	want := []string{"alpha", "bravo", "charlie"}
	for i, w := range want {
		got := filepath.Base(items[i+1].Cwd)
		if got != w {
			t.Errorf("items[%d] = %s; want %s (tie → alpha)", i+1, got, w)
		}
	}
}

func TestProjectsPruneList(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".git", "node_modules", "real-project"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p := &Projects{Roots: []string{root}, Cache: cache.New(t.TempDir())}
	items, _ := p.Items(context.Background())
	for _, it := range items {
		base := filepath.Base(it.Cwd)
		if base == ".git" || base == "node_modules" {
			t.Errorf("prune failed: %s appeared in items", base)
		}
	}
}
