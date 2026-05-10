package sesh

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestParseJSONList(t *testing.T) {
	in := []byte(`[
		{"Src":"tmux","Name":"alpha","Path":"/dev/alpha","Attached":1,"Windows":2},
		{"Src":"zoxide","Name":"~/dev/proj","Path":"/Users/me/dev/proj","Score":99},
		{"Src":"config","Name":"work","Path":"/Users/me/work"}
	]`)
	got, ok := parseJSONList(in)
	if !ok {
		t.Fatal("parseJSONList ok=false")
	}
	want := []Item{
		{Display: "● alpha", Target: "alpha", Path: "/dev/alpha", Source: "tmux", Rank: 0, Attached: 1},
		{Display: " ~/dev/proj", Target: "~/dev/proj", Path: "/Users/me/dev/proj", Source: "zoxide", Rank: 0},
		{Display: "◇ work", Target: "work", Path: "/Users/me/work", Source: "config", Rank: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseJSONList:\n got=%+v\n want=%+v", got, want)
	}
}

func TestSortItemsDevChildrenByMtime(t *testing.T) {
	home := t.TempDir()
	dev := filepath.Join(home, "dev")
	oldProj := filepath.Join(dev, "old")
	newProj := filepath.Join(dev, "new")
	other := filepath.Join(home, ".codex")
	for _, dir := range []string{oldProj, newProj, other} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Unix(100, 0)
	newTime := time.Unix(200, 0)
	if err := os.Chtimes(oldProj, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newProj, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(other, time.Unix(300, 0), time.Unix(300, 0)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	items, ok := parseJSONList([]byte(`[
		{"Src":"zoxide","Name":"~/.codex","Path":"` + other + `"},
		{"Src":"zoxide","Name":"~/dev/old","Path":"` + oldProj + `"},
		{"Src":"tmux","Name":"alpha","Path":""},
		{"Src":"zoxide","Name":"~/dev","Path":"` + dev + `"},
		{"Src":"zoxide","Name":"~/dev/new","Path":"` + newProj + `"}
	]`))
	if !ok {
		t.Fatal("parseJSONList ok=false")
	}
	sortItems(items)

	got := make([]string, len(items))
	for i, it := range items {
		got[i] = it.Target
	}
	want := []string{"alpha", "~/dev/new", "~/dev/old", "~/.codex", "~/dev"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("order = %v; want %v", got, want)
	}
}

func TestParseJSONListInvalid(t *testing.T) {
	if _, ok := parseJSONList([]byte("\x1b[34m \x1b[39m alpha\n")); ok {
		t.Fatal("parseJSONList accepted non-json icon output")
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
