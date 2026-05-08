package tmux

import (
	"context"
	"strconv"
	"strings"
)

type Session struct {
	Name         string
	Attached     bool
	LastAttached int64 // unix seconds
	Path         string
	Windows      int
}

// ListSessions runs `tmux list-sessions -F SessionFormat` and parses the
// output. If no server is running, returns (nil, nil) — empty list, no error
// (caller treats as "no sessions yet").
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	out, err := c.Output(ctx, "list-sessions", "-F", SessionFormat)
	if err != nil {
		if IsNoServer(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseSessions(out), nil
}

func parseSessions(b []byte) []Session {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	out := make([]Session, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 5 {
			continue
		}
		s := Session{
			Name: fields[0],
			Path: fields[3],
		}
		s.Attached = fields[1] != "0"
		if v, err := strconv.ParseInt(fields[2], 10, 64); err == nil {
			s.LastAttached = v
		}
		if v, err := strconv.Atoi(fields[4]); err == nil {
			s.Windows = v
		}
		out = append(out, s)
	}
	return out
}

// HasSession reports whether a session with the given name exists.
func (c *Client) HasSession(ctx context.Context, name string) bool {
	// `--` separator so dash-prefixed names don't get parsed as flags.
	cmd := c.Cmd(ctx, "has-session", "-t", "="+name)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// KillSession ends a live session. Idempotent at the user level — we don't
// surface "no such session" as an error.
func (c *Client) KillSession(ctx context.Context, name string) error {
	err := c.Run(ctx, "kill-session", "-t", "="+name)
	// kill-session returns non-zero if the session is gone. Treat as success.
	if err != nil && !c.HasSession(ctx, name) {
		return nil
	}
	return err
}

// RenameSession renames an existing session.
func (c *Client) RenameSession(ctx context.Context, oldName, newName string) error {
	return c.Run(ctx, "rename-session", "-t", "="+oldName, "--", newName)
}
