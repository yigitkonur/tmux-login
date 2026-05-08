#!/bin/sh
# Fake tmux that records its argv to $RUN_LOG (one line per call) and reads
# session/window/pane lists from canned fixture files. Drop-in PATH override
# for test/runtime.sh.
set -eu

# Record argv. NUL-separated isn't portable to all `printf`s; we use
# space-joined with a leader so multi-arg calls line up readably. The
# assertion shape is "tmux ... NAME ... -c DIR" — readable by humans too.
log_argv() {
  printf 'tmux'
  for a in "$@"; do
    case "$a" in
      *' '*|*"'"*) printf " '%s'" "$(printf '%s' "$a" | sed "s/'/'\\\\''/g")" ;;
      *)           printf ' %s' "$a" ;;
    esac
  done
  printf '\n'
}

if [ -n "${RUN_LOG:-}" ]; then
  log_argv "$@" >> "$RUN_LOG"
fi

case "${1:-}" in
  -V) echo "tmux ${MOCK_TMUX_VERSION:-3.6a}"; exit 0 ;;
esac

# Subcommand dispatch. We only emulate the read paths the binary uses.
sub=${1:-}
shift || true
case "$sub" in
  list-sessions)
    cat "${MOCK_TMUX_LIST_SESSIONS:-/dev/null}" 2>/dev/null
    exit 0
    ;;
  list-windows|list-panes)
    cat "${MOCK_TMUX_LIST_WINDOWS:-/dev/null}" 2>/dev/null
    exit 0
    ;;
  has-session|new-session|attach|attach-session|switch-client|kill-session|rename-session|detach-client|display-popup|display-message)
    # Consume args; succeed silently. The argv was already logged.
    exit 0
    ;;
  show)
    # show -gv set-clipboard
    echo "external"
    exit 0
    ;;
  source-file)
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
