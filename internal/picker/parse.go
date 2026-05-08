package picker

// Result is what we hand back from one fzf invocation.
//
// RC encodes:
//
//	0   — user picked an item; Selected is non-empty.
//	1   — no match; user pressed Enter on a non-matching query (type-to-create).
//	130 — user pressed Esc / Ctrl-C; abort silently.
//	2+  — fzf error; we surface to caller.
//
// `--print-query` emits the current query as line 1 and any selected item
// as line 2, on every exit code (including Esc). That's the rc-trap zellij-
// login closes in zellij-ssh-login.zsh:289-310; we port the same shape.
type Result struct {
	RC       int
	Query    string
	Selected string // raw fzf line (encoded\tdisplay) if RC==0
	Key      string // value from --expect=… ("" if Enter)
}

// Parsed unpacks Selected (encoded\tdisplay) into action+target+cwd.
type Parsed struct {
	Action  string
	Target  string
	Cwd     string
	Display string
	OK      bool
}

func (r Result) Parsed() Parsed {
	if r.Selected == "" {
		return Parsed{}
	}
	a, t, c, d, ok := Decode(r.Selected)
	return Parsed{Action: a, Target: t, Cwd: c, Display: d, OK: ok}
}

// IsCancelled reports whether the user pressed Esc/Ctrl-C.
func (r Result) IsCancelled() bool { return r.RC == 130 }

// IsTypeToCreate reports whether fzf returned "no match + Enter" with a
// non-empty query — the user typed something that didn't match any item and
// expects us to create it.
func (r Result) IsTypeToCreate() bool {
	return r.RC == 1 && r.Query != ""
}
