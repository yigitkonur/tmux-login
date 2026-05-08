package sources

import "testing"

func TestHumanAgo(t *testing.T) {
	cases := map[int64]string{
		5:     "5s ago",
		120:   "2m ago",
		3600:  "1h ago",
		90000: "1d ago",
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
