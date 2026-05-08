package sesh

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStripANSI(t *testing.T) {
	cases := map[string]string{
		"plain":                   "plain",
		"\x1b[34m":                "",
		"\x1b[34mhello\x1b[0m":    "hello",
		"\x1b[34m  \x1b[39m foo": "   foo",
	}
	for in, want := range cases {
		if got := stripANSI(in); got != want {
			t.Errorf("stripANSI(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestExtractTarget(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"tmux-session", "\x1b[34m  \x1b[39m alpha", "alpha", true},
		{"zoxide-path", "\x1b[36m  \x1b[39m ~/dev/proj", "~/dev/proj", true},
		{"empty", "", "", false},
		{"only-color", "\x1b[34m\x1b[39m", "", false},
		{"only-icon", "\x1b[34m \x1b[39m", "", false},
		{"no-color", "  plain-name", "plain-name", true},
		{"trailing-cr", "\x1b[34m \x1b[39m beta\r", "beta", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractTarget(tc.in)
			if ok != tc.ok {
				t.Errorf("ok=%v; want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("target=%q; want %q", got, tc.want)
			}
		})
	}
}

func TestParseList(t *testing.T) {
	in := []byte("\x1b[34m \x1b[39m alpha\n\x1b[36m \x1b[39m ~/dev/proj\n\n")
	got := parseList(in)
	want := []Item{
		{Display: "\x1b[34m \x1b[39m alpha", Target: "alpha"},
		{Display: "\x1b[36m \x1b[39m ~/dev/proj", Target: "~/dev/proj"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseList:\n got=%+v\n want=%+v", got, want)
	}
}

func TestAvailableViaSeshBin(t *testing.T) {
	// Point SESH_BIN at /bin/sh — a binary we know exists. Available()
	// should return true.
	t.Setenv("SESH_BIN", "/bin/sh")
	c := New()
	if !c.Available() {
		t.Errorf("Available() = false for /bin/sh; want true")
	}
}

func TestAvailableMissing(t *testing.T) {
	t.Setenv("SESH_BIN", "/no/such/path")
	t.Setenv("PATH", "/no/such/path") // no sesh on PATH either
	c := New()
	if c.Available() {
		t.Errorf("Available() = true for missing binary; want false")
	}
}

func TestListWithStub(t *testing.T) {
	// Build a tiny shell stub that emits canned sesh-list output.
	dir := t.TempDir()
	stub := filepath.Join(dir, "sesh-stub.sh")
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"list\" ]; then\n" +
		"  printf '\\x1b[34m \\xef\\x84\\xa0\\x1b[39m alpha\\n\\x1b[36m \\xef\\x81\\xbb\\x1b[39m ~/dev/proj\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SESH_BIN", stub)

	c := New()
	if !c.Available() {
		t.Fatalf("stub at %s not available", stub)
	}
	items, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	if items[0].Target != "alpha" || items[1].Target != "~/dev/proj" {
		t.Errorf("targets wrong: %+v", items)
	}
}
