// Package picker orchestrates the fzf side of the Universal Menu. We pipe
// items in over stdin (one per line, internal\tdisplay), bind keys to
// re-exec our binary with `_action`, and parse `--print-query` rc=0/1/130
// to distinguish selection / type-to-create / cancel.
package picker

import (
	"strings"

	"github.com/yigitkonur/tmux-login/internal/sources"
)

// Each fzf input line is: <encoded>\t<display>
// `--delimiter '\t' --with-nth 2..` hides the encoded column from the user;
// `--nth 2..` makes fuzzy search ignore it. fzf's stdout returns the full
// line (including the encoded prefix) so we recover action+target+cwd.
//
// Inside <encoded>, we separate fields with the ASCII Unit Separator (\x1f)
// because target and cwd may contain anything except tab and newline.

const us = "\x1f"

// Encode renders an Item to one fzf input line.
func Encode(it sources.Item) string {
	action := actionName(it.ActionKind)
	return action + us + it.Target + us + it.Cwd + "\t" + it.Display
}

// Decode parses one fzf-emitted line back into an action/target/cwd plus the
// remaining display text. ok=false if the line is malformed (caller should
// treat as "nothing selected").
func Decode(line string) (action, target, cwd, display string, ok bool) {
	tab := strings.IndexByte(line, '\t')
	if tab < 0 {
		return "", "", "", "", false
	}
	enc := line[:tab]
	display = line[tab+1:]
	parts := strings.SplitN(enc, us, 3)
	if len(parts) != 3 {
		return "", "", "", "", false
	}
	return parts[0], parts[1], parts[2], display, true
}

func actionName(k sources.ActionKind) string {
	switch k {
	case sources.ActionAttach:
		return "attach"
	case sources.ActionSwitchTo:
		return "switch"
	case sources.ActionSSHWindow:
		return "ssh"
	case sources.ActionShowHelp:
		return "help"
	default:
		return "noop"
	}
}

// SkipSentinel is the row pinned to the top of the SSH-login picker so
// pressing Enter on an empty query lands you in a plain shell.
const SkipSentinel = "skip"

// EncodeSkip returns the sentinel line.
func EncodeSkip() string {
	return SkipSentinel + us + us + "\t[ skip · plain shell ]"
}
