package tmux

import "context"

// HasSession reports whether a session with the given name exists.
// Used by the kill flow as a precondition (no point killing a missing
// session) and by the type-to-create attach path to detect "fresh
// create vs already exists" — see attach.go.
func (c *Client) HasSession(ctx context.Context, name string) bool {
	// `=NAME` target syntax disables fnmatch wildcards; otherwise a
	// session named e.g. `foo*` would accidentally match many sessions.
	cmd := c.Cmd(ctx, "has-session", "-t", "="+name)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// KillSession ends a live session. Idempotent at the user level — we
// don't surface "no such session" as an error.
func (c *Client) KillSession(ctx context.Context, name string) error {
	err := c.Run(ctx, "kill-session", "-t", "="+name)
	if err != nil && !c.HasSession(ctx, name) {
		// Session was already gone; treat as success.
		return nil
	}
	return err
}
