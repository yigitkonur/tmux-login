package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordRecentDirDedupAndCap(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	d1, d2 := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	for _, d := range []string{d1, d2} {
		_ = os.MkdirAll(d, 0o755)
	}

	if err := c.RecordRecentDir(d1); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordRecentDir(d2); err != nil {
		t.Fatal(err)
	}
	// Re-record d1 — should bubble back to top, dedup'd.
	if err := c.RecordRecentDir(d1); err != nil {
		t.Fatal(err)
	}

	got := c.RecentDirs()
	if len(got) != 2 {
		t.Fatalf("got %d recents; want 2", len(got))
	}
	if got[0] != d1 {
		t.Errorf("first = %q; want most-recent %q", got[0], d1)
	}
}

func TestRecentDirsFiltersDeleted(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	gone := filepath.Join(dir, "ghost")
	if err := c.RecordRecentDir(gone); err != nil {
		t.Fatal(err)
	}
	if got := c.RecentDirs(); len(got) != 0 {
		t.Errorf("RecentDirs should filter non-existent dirs, got %v", got)
	}
}
