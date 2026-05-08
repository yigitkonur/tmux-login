#!/bin/sh
# Fake sesh that records argv to $RUN_LOG and emits canned output for the
# subcommand under test. Mirrors the tmux-stub.sh / fzf-stub.sh pattern.
set -eu

# argv recorder — same shape as tmux-stub.sh so assertions compose.
log_argv() {
  printf 'sesh'
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

sub=${1:-}
case "$sub" in
  list)
    # Output canned list lines from $MOCK_SESH_LIST (path) — defaults to
    # an empty list. Real sesh emits ANSI + Nerd-Font-icon-prefixed lines;
    # the binary's parser tolerates either form, so plain names work too.
    if [ -n "${MOCK_SESH_LIST:-}" ] && [ -r "$MOCK_SESH_LIST" ]; then
      cat "$MOCK_SESH_LIST"
    fi
    exit 0
    ;;
  connect|last)
    # Succeed silently. The argv was already logged. The binary's Connect
    # uses syscall.Exec when outside tmux, so calling this stub from a
    # test outside tmux replaces the binary process; the caller sees rc=0.
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
