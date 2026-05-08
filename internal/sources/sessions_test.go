package sources

import "testing"

func TestHumanAgo(t *testing.T) {
	cases := map[int64]string{
		5:          "just now",
		59:         "just now",
		60:         "1m ago",
		120:        "2m ago",
		3600:       "1h ago",   // exact hour drops minute suffix
		3660:       "1h1m ago", // mixed precision kept (was "1h ago" before)
		7200:       "2h ago",
		7800:       "2h10m ago",
		86400:      "1d ago",
		90000:      "1d1h ago", // mixed precision past 24h
		7 * 86400:  "7d ago",   // week+ collapses to whole days
		30 * 86400: "30d ago",
	}
	for in, want := range cases {
		if got := humanAgo(in); got != want {
			t.Errorf("humanAgo(%d) = %q; want %q", in, got, want)
		}
	}
}

func TestDeriveSessionName(t *testing.T) {
	cases := map[string]string{
		"alpha":   "alpha",
		"my.proj": "my_proj",
		"v.1.2.3": "v_1_2_3",
		"":        "",
	}
	for in, want := range cases {
		if got := deriveSessionName(in); got != want {
			t.Errorf("deriveSessionName(%q) = %q; want %q", in, got, want)
		}
	}
}
