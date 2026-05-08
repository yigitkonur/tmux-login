package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordAttachAndAttachedAt(t *testing.T) {
	c := New(t.TempDir())
	if err := c.RecordAttach("alpha"); err != nil {
		t.Fatal(err)
	}
	if got := c.AttachedAt("alpha"); got == 0 {
		t.Errorf("AttachedAt(alpha) = 0; want > 0")
	}
	if got := c.AttachedAt("missing"); got != 0 {
		t.Errorf("AttachedAt(missing) = %d; want 0", got)
	}
}

func TestRecordCwd(t *testing.T) {
	c := New(t.TempDir())
	if err := c.RecordCwd("alpha", "/home/u/dev/alpha"); err != nil {
		t.Fatal(err)
	}
	if got := c.CwdOf("alpha"); got != "/home/u/dev/alpha" {
		t.Errorf("CwdOf = %q", got)
	}
	if got := c.CwdOf("missing"); got != "" {
		t.Errorf("CwdOf(missing) = %q", got)
	}
}

func TestRecordRecentDirDedupAndCap(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)

	// Create some real dirs to pass the existence filter.
	d1, d2, d3 := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	for _, d := range []string{d1, d2, d3} {
		_ = os.MkdirAll(d, 0o755)
	}

	if err := c.RecordRecentDir(d1); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordRecentDir(d2); err != nil {
		t.Fatal(err)
	}
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

func TestSnapshotRoundtrip(t *testing.T) {
	c := New(t.TempDir())
	want := []byte("alpha\t1\t1700000000\t/tmp\t1\n")
	if err := c.WriteSessionsSnapshot(want); err != nil {
		t.Fatal(err)
	}
	got := c.ReadSessionsSnapshot()
	if string(got) != string(want) {
		t.Errorf("snapshot mismatch:\nwant %q\ngot  %q", want, got)
	}
}

func TestGC(t *testing.T) {
	c := New(t.TempDir())
	_ = c.RecordAttach("alpha")
	_ = c.RecordAttach("beta")
	_ = c.RecordCwd("alpha", "/tmp")
	c.GC([]string{"alpha"}, true)
	if c.AttachedAt("beta") != 0 {
		t.Error("beta attached marker should have been GC'd")
	}
	if c.AttachedAt("alpha") == 0 {
		t.Error("alpha attached marker should have been kept")
	}
	if c.CwdOf("alpha") != "/tmp" {
		t.Error("alpha cwd marker should have been kept")
	}
}

func TestGCDoesNotTouchOnNoLive(t *testing.T) {
	c := New(t.TempDir())
	_ = c.RecordAttach("alpha")
	// hasLive=false → don't sweep
	c.GC(nil, false)
	if c.AttachedAt("alpha") == 0 {
		t.Error("alpha should still be present when hasLive=false")
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

// regression: AttachedAt's mtime should be monotonically increasing across
// successive RecordAttach calls — used by the sort.
func TestAttachedAtMonotonic(t *testing.T) {
	c := New(t.TempDir())
	_ = c.RecordAttach("alpha")
	t1 := c.AttachedAt("alpha")
	time.Sleep(1100 * time.Millisecond)
	_ = c.RecordAttach("alpha")
	t2 := c.AttachedAt("alpha")
	if t2 <= t1 {
		t.Errorf("re-record didn't bump mtime: t1=%d t2=%d", t1, t2)
	}
}
