// Package install holds marker-block constants and idempotent file-mutation
// helpers used by both `tmux-login install-hooks` (Go) and install.sh (POSIX
// sh, via the binary). The Go side is authoritative — the shell scripts
// invoke `tmux-login install-hooks` rather than re-implementing the awk.
package install

const (
	MarkOpen             = "# tmux-login:hook {{{"
	MarkClose            = "# tmux-login:hook }}}"
	MarkNoFinalNewline   = "# tmux-login:original-no-final-newline"
	StateDirRel          = "tmux-login"
	StateInstallJSONName = "install.json"
	TmuxConfBasename     = ".tmux.conf"
	ZshrcBasename        = ".zshrc"
	ManagedTmuxConfRel   = "share/tmux.conf"
	ManagedLoginHookRel  = "share/login-hook.zsh"
)
