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

func TestSessionMetadata(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	cwd := filepath.Join(dir, "proj")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := c.RecordAttach("alpha"); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordCwd("alpha", cwd); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "attached", "alpha")); err != nil {
		t.Fatalf("attached marker missing: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "cwds", "alpha"))
	if err != nil {
		t.Fatalf("cwd marker missing: %v", err)
	}
	if string(got) != cwd+"\n" {
		t.Fatalf("cwd marker = %q; want %q", got, cwd+"\n")
	}

	c.DropSession("alpha")
	if _, err := os.Stat(filepath.Join(dir, "attached", "alpha")); !os.IsNotExist(err) {
		t.Fatalf("attached marker still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cwds", "alpha")); !os.IsNotExist(err) {
		t.Fatalf("cwd marker still present: %v", err)
	}
}

func TestSessionMetadataRejectsUnsafeNames(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	if err := c.RecordAttach("../bad"); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordCwd("bad/name", "/tmp"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "attached")); !os.IsNotExist(err) {
		t.Fatalf("unsafe attach created metadata dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cwds")); !os.IsNotExist(err) {
		t.Fatalf("unsafe cwd created metadata dir: %v", err)
	}
}
