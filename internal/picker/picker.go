package picker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Spawner runs an external process with stdin from the items pipe; tests
// can substitute. Default uses os/exec via fzf on PATH.
type Spawner func(ctx context.Context, args []string, stdin []byte, env []string) (stdout []byte, rc int, err error)

// Spec describes one fzf invocation.
type Spec struct {
	Prompt string
	Header string
	Lines  []string // already-encoded item lines (and any sentinels at desired position)
	Expect []string // keys we want fzf to capture and return as the first line
	Binds  []string // each is a complete fzf --bind argument value (e.g. "ctrl-x:execute-silent(...)+abort")
	// PrintQuery enables the rc-trap. The picker always wants this true; left
	// as a field so tests can assert.
	PrintQuery bool
	// FZFBin is the fzf binary path. "" means PATH lookup.
	FZFBin string
}

// Pick runs fzf with the spec and returns a Result. spawn=nil uses defaultSpawner.
func Pick(ctx context.Context, spec Spec, spawn Spawner) (Result, error) {
	if spawn == nil {
		spawn = DefaultSpawner
	}
	args := buildArgs(spec)
	stdin := []byte(strings.Join(spec.Lines, "\n") + "\n")

	binPath := spec.FZFBin
	if binPath == "" {
		binPath = "fzf"
	}

	full := append([]string{binPath}, args...)
	stdout, rc, err := spawn(ctx, full, stdin, os.Environ())
	if err != nil && rc != 130 && rc != 1 && rc != 0 {
		return Result{RC: rc}, err
	}

	r := Result{RC: rc}
	parseStdout(stdout, spec, &r)
	return r, nil
}

func parseStdout(out []byte, spec Spec, r *Result) {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) == 0 {
		return
	}
	idx := 0
	if len(spec.Expect) > 0 && len(lines) > idx {
		r.Key = lines[idx]
		idx++
	}
	if spec.PrintQuery && len(lines) > idx {
		r.Query = lines[idx]
		idx++
	}
	if r.RC == 0 && len(lines) > idx {
		r.Selected = lines[idx]
	}
}

// DefaultSpawner runs fzf via os/exec. Returns stdout, exit code, and
// error. When fzf exits non-zero (e.g. rc=1 for no-match-Enter, rc=130 for
// Esc) the error is the *exec.ExitError but we capture rc into the int.
func DefaultSpawner(ctx context.Context, args []string, stdin []byte, env []string) ([]byte, int, error) {
	if len(args) == 0 {
		return nil, 2, fmt.Errorf("picker: empty args")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return out.Bytes(), ee.ExitCode(), nil
		}
		return out.Bytes(), -1, err
	}
	return out.Bytes(), 0, nil
}
