package tmux

import (
	"context"
	"strconv"
	"strings"
)

type Window struct {
	Session string
	Index   int
	ID      string
	Name    string
	Active  bool
	Command string
}

// ListWindows runs `tmux list-windows -aF WindowFormat` (-a = all sessions).
func (c *Client) ListWindows(ctx context.Context) ([]Window, error) {
	out, err := c.Output(ctx, "list-windows", "-a", "-F", WindowFormat)
	if err != nil {
		if IsNoServer(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseWindows(out), nil
}

func parseWindows(b []byte) []Window {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	out := make([]Window, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 6)
		if len(fields) < 6 {
			continue
		}
		w := Window{
			Session: fields[0],
			ID:      fields[2],
			Name:    fields[3],
			Active:  fields[4] == "1",
			Command: fields[5],
		}
		if v, err := strconv.Atoi(fields[1]); err == nil {
			w.Index = v
		}
		out = append(out, w)
	}
	return out
}
