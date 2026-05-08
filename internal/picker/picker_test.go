package picker

import (
	"context"
	"os/exec"
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
		// Regression: --nth=2.. must NOT be present (broke search on 2-field lines).
		if strings.Contains(joined, "--nth=") {
			t.Errorf("args should not include --nth (breaks 2-field matching): %s", joined)
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

// TestPickQueryPrefill: when Spec.Query is set, fzf gets --query=VALUE so
// the user lands in the picker with the filter already applied.
func TestPickQueryPrefill(t *testing.T) {
	spec := Spec{Lines: []string{"x\ty"}, Query: "testo", PrintQuery: true}
	stub := func(ctx context.Context, args []string, stdin []byte, env []string) ([]byte, int, error) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--query=testo") {
			t.Errorf("args missing --query=testo: %s", joined)
		}
		return []byte("testo\n"), 1, nil // rc=1 = no match, query echoed back
	}
	r, _ := Pick(context.Background(), spec, stub)
	if !r.IsTypeToCreate() || r.Query != "testo" {
		t.Errorf("expected type-to-create with query=testo; got %+v", r)
	}
}

// TestRealFZFMatching is the regression net for the args→fzf interaction.
// Stub-driven tests proved that --print-query rc-trap works at the binary
// level, but we shipped a bug where --with-nth=2.. + --nth=2.. on
// 2-field lines silently returned zero matches (issue caught after the
// dir-picker basename-column was dropped). This test invokes real fzf in
// --filter mode against the args we'd ship and asserts the search hits.
func TestRealFZFMatching(t *testing.T) {
	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not on PATH; skipping real-fzf integration test")
	}

	cases := []struct {
		name  string
		lines []string // fzf input
		query string
		want  string // expected substring in fzf output
	}{
		{
			name: "2-field projects line",
			lines: []string{
				"attach\x1fcodex-lb\x1f/U/yk/dev/codex-lb\t~/dev/codex-lb",
				"attach\x1fdocs\x1f/U/yk/dev/docs\t~/dev/docs",
			},
			query: "codex",
			want:  "~/dev/codex-lb",
		},
		{
			name: "4-field sessions line",
			lines: []string{
				"attach\x1falpha\x1f/dev/alpha\t●\talpha\t(3w, 5m ago)",
				"attach\x1fbeta\x1f/dev/beta\t○\tbeta\t(1w, 2h ago)",
			},
			query: "alpha",
			want:  "alpha",
		},
		{
			name: "single-letter substring on 2-field",
			lines: []string{
				"attach\x1fa\x1f/p\t~/dev/codex",
				"attach\x1fb\x1f/p\t~/dev/iptv",
			},
			query: "c",
			want:  "codex",
		},
		{
			name: "encoded field 1 must NOT leak into search",
			lines: []string{
				"attach\x1fSECRETWORD\x1f/p\t~/dev/proj",
			},
			query: "SECRETWORD",
			want:  "", // expect zero matches
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := buildArgs(Spec{}) // no print-query, no expect — pure args
			args = append(args, "--filter="+tc.query)
			cmd := exec.Command("fzf", args...)
			cmd.Stdin = strings.NewReader(strings.Join(tc.lines, "\n") + "\n")
			out, _ := cmd.Output() // rc=1 on no match is fine; we just check stdout
			got := string(out)
			if tc.want == "" {
				if strings.TrimSpace(got) != "" {
					t.Errorf("expected no matches; got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("query %q: expected output to contain %q; got %q",
					tc.query, tc.want, got)
			}
		})
	}
}
