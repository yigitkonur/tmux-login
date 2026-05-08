package picker

import (
	"context"
	"strings"
	"testing"

	"github.com/yigitkonur/tmux-login/internal/sources"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	cases := []sources.Item{
		{ActionKind: sources.ActionAttach, Target: "alpha", Cwd: "/home/u/dev/alpha", Display: "● alpha (3w)"},
		{ActionKind: sources.ActionSwitchTo, Target: "alpha:1", Cwd: "", Display: "alpha:1 nvim"},
		{ActionKind: sources.ActionAttach, Target: "name with spaces", Cwd: "/home/u/dev/x y", Display: "with spaces (1w)"},
	}
	for _, in := range cases {
		line := Encode(in)
		a, tg, c, _, ok := Decode(line)
		if !ok {
			t.Errorf("decode of %q failed", line)
			continue
		}
		if tg != in.Target || c != in.Cwd || a != actionName(in.ActionKind) {
			t.Errorf("roundtrip diff: got action=%s target=%s cwd=%s", a, tg, c)
		}
	}
}

func TestDecodeMalformed(t *testing.T) {
	if _, _, _, _, ok := Decode(""); ok {
		t.Error("empty line should not decode")
	}
	if _, _, _, _, ok := Decode("no-tab-line"); ok {
		t.Error("missing tab should not decode")
	}
	if _, _, _, _, ok := Decode("only-one-field\tdisplay"); ok {
		t.Error("missing US separators should not decode")
	}
}

func TestParseStdoutWithExpectAndQuery(t *testing.T) {
	spec := Spec{Expect: []string{"ctrl-x", "ctrl-r"}, PrintQuery: true}
	r := Result{RC: 0}
	parseStdout([]byte("ctrl-x\nfoo\nattack\x1ftarget\x1fcwd\t● target\n"), spec, &r)
	if r.Key != "ctrl-x" || r.Query != "foo" || !strings.Contains(r.Selected, "target") {
		t.Errorf("parsed = %+v", r)
	}
}

func TestParseStdoutTypeToCreate(t *testing.T) {
	spec := Spec{PrintQuery: true}
	r := Result{RC: 1}
	parseStdout([]byte("newproj\n"), spec, &r)
	if r.Query != "newproj" {
		t.Errorf("query = %q; want newproj", r.Query)
	}
	if !r.IsTypeToCreate() {
		t.Error("IsTypeToCreate should be true")
	}
}

func TestParseStdoutEsc(t *testing.T) {
	spec := Spec{PrintQuery: true}
	r := Result{RC: 130}
	parseStdout([]byte("typed-this\n"), spec, &r)
	if !r.IsCancelled() {
		t.Error("rc=130 should report cancelled")
	}
	if r.IsTypeToCreate() {
		t.Error("rc=130 must not fall into type-to-create")
	}
}

func TestPickWithStubSpawner(t *testing.T) {
	spec := Spec{
		Prompt:     "test > ",
		Header:     "[hdr]",
		Lines:      []string{"attach\x1falpha\x1f/dev\t● alpha"},
		Expect:     []string{"ctrl-x"},
		PrintQuery: true,
	}
	stub := func(ctx context.Context, args []string, stdin []byte, env []string) ([]byte, int, error) {
		// Verify args contain key flags.
		joined := strings.Join(args, " ")
		for _, want := range []string{"--print-query", "--with-nth=2..", "--prompt=test > ", "--expect=ctrl-x"} {
			if !strings.Contains(joined, want) {
				t.Errorf("args missing %q: %s", want, joined)
			}
		}
		// Simulate user pressing Enter on the only item.
		return []byte("\nfoo\nattach\x1falpha\x1f/dev\t● alpha\n"), 0, nil
	}
	r, err := Pick(context.Background(), spec, stub)
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	if r.RC != 0 {
		t.Errorf("rc=%d; want 0", r.RC)
	}
	p := r.Parsed()
	if !p.OK || p.Target != "alpha" || p.Cwd != "/dev" || p.Action != "attach" {
		t.Errorf("parsed = %+v", p)
	}
}
