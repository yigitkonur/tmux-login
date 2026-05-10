package login

import "testing"

func TestCleanDirCreateName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"new-project", "new-project"},
		{"  new-project  ", "new-project"},
		{"", ""},
		{"   ", ""},
		{".", ""},
		{"..", ""},
		{"../escape", ""},
		{"nested/project", ""},
		{`nested\project`, ""},
		{"/tmp/escape", ""},
		{"bad\x00name", ""},
	}

	for _, tc := range cases {
		if got := cleanDirCreateName(tc.in); got != tc.want {
			t.Fatalf("cleanDirCreateName(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
