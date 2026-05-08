package tmux

import "testing"

func TestParseSessions(t *testing.T) {
	in := []byte("alpha\t1\t1700000000\t/home/u/dev/alpha\t3\nbeta\t0\t0\t/tmp\t1\n")
	got := parseSessions(in)
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2: %+v", len(got), got)
	}
	if got[0].Name != "alpha" || !got[0].Attached || got[0].Windows != 3 || got[0].Path != "/home/u/dev/alpha" {
		t.Errorf("sess[0] = %+v", got[0])
	}
	if got[1].Name != "beta" || got[1].Attached || got[1].LastAttached != 0 {
		t.Errorf("sess[1] = %+v", got[1])
	}
}

func TestParseSessionsEmpty(t *testing.T) {
	if got := parseSessions(nil); len(got) != 0 {
		t.Errorf("empty input returned %d sessions", len(got))
	}
	if got := parseSessions([]byte("")); len(got) != 0 {
		t.Errorf("empty string returned %d sessions", len(got))
	}
}

func TestParseSessionsMalformed(t *testing.T) {
	// missing fields — should be skipped, not crash
	in := []byte("alpha\t1\nbroken\t1\t2\nbeta\t1\t1700000000\t/tmp\t2\n")
	got := parseSessions(in)
	if len(got) != 1 || got[0].Name != "beta" {
		t.Errorf("got %+v; want only beta", got)
	}
}

func TestParseWindows(t *testing.T) {
	in := []byte("alpha\t1\t@1\tedit\t1\tnvim\nalpha\t2\t@2\tshell\t0\tzsh\n")
	got := parseWindows(in)
	if len(got) != 2 {
		t.Fatalf("got %d windows", len(got))
	}
	if !got[0].Active || got[0].Name != "edit" || got[0].Command != "nvim" {
		t.Errorf("win[0] = %+v", got[0])
	}
	if got[1].Active {
		t.Errorf("win[1] should not be active: %+v", got[1])
	}
}

func TestParsePanes(t *testing.T) {
	in := []byte("alpha\t1\t1\t%5\tnvim\t/home/u/dev/alpha\nbeta\t1\t2\t%6\tzsh\t/tmp\n")
	got := parsePanes(in)
	if len(got) != 2 {
		t.Fatalf("got %d panes", len(got))
	}
	if got[0].WindowIndex != 1 || got[0].PaneIndex != 1 || got[0].ID != "%5" {
		t.Errorf("pane[0] = %+v", got[0])
	}
	if got[1].Path != "/tmp" {
		t.Errorf("pane[1] path = %s", got[1].Path)
	}
}
