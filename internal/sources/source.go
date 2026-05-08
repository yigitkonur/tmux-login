// Package sources holds the project source for the type-to-create dir
// picker. With sesh as the session engine, the only source we still own
// is the project-dir walker (sesh has no equivalent for "show me the
// candidate dirs at the time the user is naming a new project").
//
// Item is a thin shape used by both projects.go and the picker's
// encoding layer (internal/picker/encoding.go).
package sources

type Mode string

const ModeProjects Mode = "projects"

type ActionKind int

// Only ActionAttach is in use post-v0.3. Earlier versions had multiple
// action kinds; sesh subsumes them.
const ActionAttach ActionKind = 0

// Item is one row in the dir picker.
type Item struct {
	Mode       Mode
	Display    string // line shown in fzf
	ActionKind ActionKind
	Target     string // session name to attach as
	Cwd        string // start dir for the new session
}
