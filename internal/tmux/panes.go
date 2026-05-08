package tmux

import (
	"context"
	"strconv"
	"strings"
)

type Pane struct {
	Session     string
	WindowIndex int
	PaneIndex   int
	ID          string
	Command     string
	Path        string
}

func (c *Client) ListPanes(ctx context.Context) ([]Pane, error) {
	out, err := c.Output(ctx, "list-panes", "-a", "-F", PaneFormat)
	if err != nil {
		if IsNoServer(err) {
			return nil, nil
		}
		return nil, err
	}
	return parsePanes(out), nil
}

func parsePanes(b []byte) []Pane {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	out := make([]Pane, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 6)
		if len(fields) < 6 {
			continue
		}
		p := Pane{
			Session: fields[0],
			ID:      fields[3],
			Command: fields[4],
			Path:    fields[5],
		}
		if v, err := strconv.Atoi(fields[1]); err == nil {
			p.WindowIndex = v
		}
		if v, err := strconv.Atoi(fields[2]); err == nil {
			p.PaneIndex = v
		}
		out = append(out, p)
	}
	return out
}
