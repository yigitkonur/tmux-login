#!/bin/sh
# Fake fzf for test/runtime.sh. Reads stdin (the candidate list), discards
# it (or captures it if FZF_STDIN_DIR is set), and emits a canned response.
#
# Per-call indexing: each call increments a counter file; outputs come from
# $FZF_OUTPUTS_DIR/<idx>; rc from $FZF_RC_DIR/<idx>; stdin gets dumped to
# $FZF_STDIN_DIR/<idx>.
set -eu

state_dir=${FZF_STATE_DIR:-${TMPDIR:-/tmp}/fzf-stub-state}
mkdir -p -- "$state_dir"
counter_file="$state_dir/counter"

# Read-modify-write counter; portable enough for the test harness.
if [ -f "$counter_file" ]; then
  idx=$(cat "$counter_file")
else
  idx=0
fi
idx=$((idx + 1))
echo "$idx" > "$counter_file"

# Capture stdin if asked.
if [ -n "${FZF_STDIN_DIR:-}" ]; then
  mkdir -p -- "$FZF_STDIN_DIR"
  cat > "$FZF_STDIN_DIR/$idx"
else
  cat > /dev/null
fi

# Determine rc.
rc=0
if [ -n "${FZF_RC_DIR:-}" ] && [ -f "$FZF_RC_DIR/$idx" ]; then
  rc=$(cat "$FZF_RC_DIR/$idx")
fi

# Emit canned output.
if [ -n "${FZF_OUTPUTS_DIR:-}" ] && [ -f "$FZF_OUTPUTS_DIR/$idx" ]; then
  cat "$FZF_OUTPUTS_DIR/$idx"
fi

exit "$rc"
