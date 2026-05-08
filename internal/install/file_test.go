package install

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureMarkerFreshFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rc")
	added, err := EnsureMarker(p, []string{"echo hi"})
	if err != nil || !added {
		t.Fatalf("EnsureMarker: added=%v err=%v", added, err)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Contains(got, []byte(MarkOpen)) || !bytes.Contains(got, []byte("echo hi")) || !bytes.Contains(got, []byte(MarkClose)) {
		t.Errorf("file content = %q", got)
	}
}

func TestEnsureMarkerIdempotent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rc")
	_, _ = EnsureMarker(p, []string{"echo hi"})
	added, err := EnsureMarker(p, []string{"echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Error("second call should be no-op")
	}
	got, _ := os.ReadFile(p)
	count := bytes.Count(got, []byte(MarkOpen))
	if count != 1 {
		t.Errorf("MarkOpen count = %d; want 1", count)
	}
}

func TestEnsureAndStripByteForByte(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rc")
	original := []byte("alias ll='ls -la'\nexport PATH=$PATH:/foo\n")
	if err := os.WriteFile(p, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureMarker(p, []string{"source /foo/bar"}); err != nil {
		t.Fatal(err)
	}
	if _, err := StripMarker(p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, original) {
		t.Errorf("byte-for-byte restore failed:\norig: %q\ngot:  %q", original, got)
	}
}

func TestEnsureAndStripNoFinalNewline(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rc")
	original := []byte("alias ll='ls -la'\nexport PATH=$PATH:/foo")
	if err := os.WriteFile(p, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureMarker(p, []string{"source /foo/bar"}); err != nil {
		t.Fatal(err)
	}
	if _, err := StripMarker(p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, original) {
		t.Errorf("byte-for-byte restore (no-final-newline) failed:\norig: %q\ngot:  %q", original, got)
	}
}

func TestStripMissingFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rc")
	stripped, err := StripMarker(p)
	if err != nil || stripped {
		t.Errorf("missing file: stripped=%v err=%v", stripped, err)
	}
}

func TestQuoteShell(t *testing.T) {
	cases := map[string]string{
		"plain":      "'plain'",
		"with space": "'with space'",
		"sq's":       `'sq'\''s'`,
		"$dollar":    "'$dollar'",
		`back\slash`: `'back\slash'`,
		"":           "''",
	}
	for in, want := range cases {
		if got := QuoteShell(in); got != want {
			t.Errorf("QuoteShell(%q) = %q; want %q", in, got, want)
		}
	}
}
