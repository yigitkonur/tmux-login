// Package sources composes the items shown in each Universal Menu mode.
// Each Source returns a []Item; the picker handles rendering and dispatch.
//
// Items carry a tab-delimited Display line (icon + name + meta) and an
// opaque Action handle that the picker resolves to a tmux command when the
// user picks Enter / ctrl-x / etc.
package sources

import "context"

type Mode string

const (
	ModeSessions Mode = "sessions"
	ModeWindows  Mode = "windows"
	ModePanes    Mode = "panes"
	ModeProjects Mode = "projects"
	ModeSSH      Mode = "ssh"
	ModeRecent   Mode = "recent"
	ModeKill     Mode = "kill"
	ModeHelp     Mode = "help"
)

// Item is one row in the picker.
//
// ActionKind is a small enum so the picker can dispatch on it cheaply
// without holding closures across process boundaries (we re-exec for
// fzf bindings, so closures wouldn't survive).
type Item struct {
	Mode       Mode
	Display    string // line shown in fzf
	ActionKind ActionKind
	Target     string // session name, "session:idx", path, ssh alias, etc.
	Cwd        string // start dir for ActionAttach (optional)
}

type ActionKind int

const (
	ActionAttach    ActionKind = iota // attach session named Target (create at Cwd if missing)
	ActionSwitchTo                    // switch-client -t Target (window/pane id)
	ActionSSHWindow                   // open new window running `ssh Target`
	ActionShowHelp                    // help mode no-op (selection just closes)
)

type Source interface {
	Mode() Mode
	Items(ctx context.Context) ([]Item, error)
}
