#!/bin/sh
# Runtime tests for tmux-login. Replaces tmux + fzf on PATH with stubs that
# record argv and emit canned outputs, then drives the Go binary and asserts
# the exact tmux argv shape.
set -eu

# shellcheck disable=SC1007
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck disable=SC1007
REPO=$(CDPATH='' cd -- "$SCRIPT_DIR/.." && pwd)
BIN="$REPO/bin/tmux-login"
FIX="$SCRIPT_DIR/fixtures"

if [ ! -x "$BIN" ]; then
  echo "test/runtime.sh: $BIN not built; run 'make build' first" >&2
  exit 1
fi

PASS=0
FAIL=0

setup() {
  tmp=$(mktemp -d /tmp/tmux-login-rt.XXXXXX)
  shimdir="$tmp/shim"
  mkdir -p -- "$shimdir"
  cp "$FIX/tmux-stub.sh" "$shimdir/tmux"
  cp "$FIX/fzf-stub.sh"  "$shimdir/fzf"
  chmod +x "$shimdir/tmux" "$shimdir/fzf"

  RUN_LOG="$tmp/tmux.argv.log"
  : > "$RUN_LOG"

  FZF_OUTPUTS_DIR="$tmp/fzf-out"
  FZF_RC_DIR="$tmp/fzf-rc"
  FZF_STDIN_DIR="$tmp/fzf-stdin"
  FZF_STATE_DIR="$tmp/fzf-state"
  mkdir -p -- "$FZF_OUTPUTS_DIR" "$FZF_RC_DIR" "$FZF_STDIN_DIR" "$FZF_STATE_DIR"

  # Sandbox HOME so cache writes don't leak into the user's tree.
  HOME=$tmp
  ZDOTDIR=$tmp
  XDG_CACHE_HOME=$tmp/.cache
  XDG_STATE_HOME=$tmp/.local/state
  XDG_DATA_HOME=$tmp/.local/share
  XDG_CONFIG_HOME=$tmp/.config

  # Critical: clear $TMUX so InsideTmux() reads false. Otherwise the login
  # flow short-circuits silently and we never see argv.
  TMUX=""

  # Block the SSH-login skip env so the binary's defense-in-depth doesn't
  # bail (we're testing the inside; the zsh hook isn't in this test).
  TMUX_LOGIN_SKIP=""

  PATH="$shimdir:$PATH"
  export RUN_LOG FZF_OUTPUTS_DIR FZF_RC_DIR FZF_STDIN_DIR FZF_STATE_DIR \
         HOME ZDOTDIR XDG_CACHE_HOME XDG_STATE_HOME XDG_DATA_HOME XDG_CONFIG_HOME \
         TMUX TMUX_LOGIN_SKIP PATH
}

teardown() {
  rm -rf -- "$tmp"
}

run_case() {
  name=$1
  shift
  printf '  case: %-55s ' "$name"
  if (set -e; "$@") >/tmp/tmux-login-runtime.log 2>&1; then
    printf 'OK\n'
    PASS=$((PASS + 1))
  else
    printf 'FAIL\n'
    FAIL=$((FAIL + 1))
    echo '----- log -----' >&2
    cat /tmp/tmux-login-runtime.log >&2
    echo '--------------------' >&2
  fi
}

# assert_argv_contains LOG WANT
# Asserts that LOG contains a line where every word in WANT appears in order.
assert_argv_line_has() {
  log=$1
  want=$2
  if ! grep -F -- "$want" "$log" >/dev/null 2>&1; then
    echo "argv log lacks line containing: $want" >&2
    echo "--- argv recorded: ---" >&2
    cat "$log" >&2
    return 1
  fi
}

# Each canned output line is what fzf would print, including the leading
# expect-key line if --expect was passed, the query line (--print-query),
# and the selected line.

# --- 1. attach-existing -----------------------------------------------------
case_attach_existing() {
  setup
  # Pretend tmux has one session, named alpha at /tmp.
  echo "alpha	1	1700000000	/tmp	2" > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  export MOCK_TMUX_LIST_SESSIONS

  # fzf call 1: user picks the alpha row.
  printf '\nattach\x1falpha\x1f/tmp\t● alpha (2w, 5m ago)\n' > "$FZF_OUTPUTS_DIR/1"
  echo 0 > "$FZF_RC_DIR/1"

  "$BIN" login

  assert_argv_line_has "$RUN_LOG" "new-session -A -d -s alpha -c /tmp" || return 1
  # Outside tmux: we exec attach. But syscall.Exec replaces this very process —
  # in the test, the stub's tmux is found and ignores the request, so we see
  # the argv line from the EXEC'd tmux instead (recorded by the stub).
  assert_argv_line_has "$RUN_LOG" "attach -t =alpha" || return 1
  teardown
}

# --- 2. session name with spaces -------------------------------------------
case_session_name_with_spaces() {
  setup
  echo "my session	1	1700000000	/tmp	1" > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  export MOCK_TMUX_LIST_SESSIONS

  printf "\nattach\x1fmy session\x1f/tmp\t● my session (1w)\n" > "$FZF_OUTPUTS_DIR/1"
  echo 0 > "$FZF_RC_DIR/1"

  "$BIN" login

  # We log spaces via single-quoting; the assertion looks for the raw form.
  assert_argv_line_has "$RUN_LOG" "new-session -A -d -s 'my session'" || return 1
  teardown
}

# --- 3. type-to-create ------------------------------------------------------
case_type_to_create() {
  setup
  : > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  export MOCK_TMUX_LIST_SESSIONS

  # fzf #1: --print-query rc=1 with query "newproj"
  echo "newproj" > "$FZF_OUTPUTS_DIR/1"
  echo 1 > "$FZF_RC_DIR/1"
  # fzf #2: dir picker — user picks /home/u/dev/newproj
  printf '\nattach\x1fnewproj\x1f%s\tnewproj\t~/dev/newproj\n' "$tmp/dev/newproj" > "$FZF_OUTPUTS_DIR/2"
  echo 0 > "$FZF_RC_DIR/2"

  mkdir -p "$tmp/dev/newproj"

  "$BIN" login

  assert_argv_line_has "$RUN_LOG" "new-session -A -d -s newproj -c $tmp/dev/newproj" || return 1
  teardown
}

# --- 3b. type-to-create with no matching dir (auto-mkdir) ------------------
case_type_to_create_auto_mkdir() {
  setup
  : > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  TMUX_LOGIN_ROOTS="$tmp/dev"
  export MOCK_TMUX_LIST_SESSIONS TMUX_LOGIN_ROOTS

  # First root must exist so it's adopted as the auto-mkdir base.
  mkdir -p "$tmp/dev"

  # fzf #1: --print-query rc=1 with query "testo"
  echo "testo" > "$FZF_OUTPUTS_DIR/1"
  echo 1 > "$FZF_RC_DIR/1"
  # fzf #2: also rc=1 (zero-match) with the pre-filled query echoed back.
  echo "testo" > "$FZF_OUTPUTS_DIR/2"
  echo 1 > "$FZF_RC_DIR/2"

  # Pre-condition: testo must NOT exist yet.
  [ -d "$tmp/dev/testo" ] && { echo "test setup wrong: $tmp/dev/testo already exists"; return 1; }

  "$BIN" login

  # Post-condition: dir was auto-created, session attached at it.
  [ -d "$tmp/dev/testo" ] || { echo "auto-mkdir failed: $tmp/dev/testo not created"; return 1; }
  assert_argv_line_has "$RUN_LOG" "new-session -A -d -s testo -c $tmp/dev/testo" || return 1
  teardown
}

# --- 4. dash-prefixed name --------------------------------------------------
case_dash_prefixed_name() {
  setup
  : > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  export MOCK_TMUX_LIST_SESSIONS

  echo "-foo" > "$FZF_OUTPUTS_DIR/1"
  echo 1 > "$FZF_RC_DIR/1"
  printf '\nattach\x1f-foo\x1f%s\tfoo\t~/dev/foo\n' "$tmp/dev/foo" > "$FZF_OUTPUTS_DIR/2"
  echo 0 > "$FZF_RC_DIR/2"

  mkdir -p "$tmp/dev/foo"

  "$BIN" login

  # The session name "-foo" must reach tmux as a target via -s; if it leaked
  # into a flag slot the stub would swallow it as -f -o -o.
  assert_argv_line_has "$RUN_LOG" "new-session -A -d -s -foo -c" || return 1
  teardown
}

# --- 5. esc-with-query (rc=130 + non-empty query) --------------------------
case_esc_with_query() {
  setup
  : > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  export MOCK_TMUX_LIST_SESSIONS

  echo "newproj" > "$FZF_OUTPUTS_DIR/1"
  echo 130 > "$FZF_RC_DIR/1"

  "$BIN" login

  # Critical: rc=130 must NOT trigger create-new. Argv log should have only
  # list-sessions (preview reads), zero new-session calls.
  if grep -Fq "new-session" "$RUN_LOG"; then
    echo "rc=130 triggered new-session — type-to-create trap broken"
    return 1
  fi
  teardown
}

# --- 6. TMUX_LOGIN_SKIP=1 short-circuits -----------------------------------
case_skip_short_circuit() {
  setup
  TMUX_LOGIN_SKIP=1 export TMUX_LOGIN_SKIP

  "$BIN" login

  if [ -s "$RUN_LOG" ]; then
    echo "TMUX_LOGIN_SKIP=1 should fire no tmux calls; got:"
    cat "$RUN_LOG"
    return 1
  fi
  teardown
}

# --- 7. inside-tmux short-circuit ------------------------------------------
case_inside_tmux_short_circuit() {
  setup
  TMUX="/private/tmp/tmux-501/default,12345,0" export TMUX

  "$BIN" login

  if [ -s "$RUN_LOG" ]; then
    echo "inside-tmux should fire no tmux calls; got:"
    cat "$RUN_LOG"
    return 1
  fi
  teardown
}

# --- 8. attach subcommand respects --cwd -----------------------------------
case_attach_with_cwd() {
  setup
  "$BIN" attach --cwd /tmp/x --detach myproj
  assert_argv_line_has "$RUN_LOG" "new-session -A -d -s myproj -c /tmp/x" || return 1
  teardown
}

# --- 9. doctor exits 0 with expected keys ----------------------------------
case_doctor_shape() {
  setup
  out=$("$BIN" doctor 2>&1)
  for key in "tmux-login:" "tmux:" "fzf:" "inside-tmux:" "cache-dir:" "marker-tmux-conf:" "marker-zshrc:"; do
    echo "$out" | grep -Fq "$key" || { echo "doctor output missing $key"; echo "$out"; return 1; }
  done
  teardown
}

# --- 10. install-hooks --dry-run prints diff but does not write -----------
case_install_hooks_dry_run() {
  setup
  out=$("$BIN" install-hooks --tmux --zsh --dry-run 2>&1)
  echo "$out" | grep -Fq "would append" || { echo "dry-run output unexpected: $out"; return 1; }
  [ -f "$HOME/.tmux.conf" ] && { echo "dry-run wrote .tmux.conf"; return 1; }
  [ -f "$HOME/.zshrc" ] && { echo "dry-run wrote .zshrc"; return 1; }
  teardown
}

# --- 11. install-hooks idempotent ------------------------------------------
case_install_hooks_idempotent() {
  setup
  "$BIN" install-hooks --tmux --zsh
  "$BIN" install-hooks --tmux --zsh
  count=$(grep -Fc "tmux-login:hook {{{" "$HOME/.tmux.conf")
  [ "$count" = "1" ] || { echo "tmux.conf marker count = $count, want 1"; return 1; }
  count=$(grep -Fc "tmux-login:hook {{{" "$HOME/.zshrc")
  [ "$count" = "1" ] || { echo "zshrc marker count = $count, want 1"; return 1; }
  teardown
}

echo "test/runtime.sh: running cases ..."
run_case "attach-existing"                  case_attach_existing
run_case "session-name-with-spaces"         case_session_name_with_spaces
run_case "type-to-create"                   case_type_to_create
run_case "type-to-create-auto-mkdir"        case_type_to_create_auto_mkdir
run_case "dash-prefixed-name"               case_dash_prefixed_name
run_case "esc-with-query (no create)"       case_esc_with_query
run_case "TMUX_LOGIN_SKIP=1 short-circuit"  case_skip_short_circuit
run_case "inside-tmux short-circuit"        case_inside_tmux_short_circuit
run_case "attach --cwd --detach"            case_attach_with_cwd
run_case "doctor output shape"              case_doctor_shape
run_case "install-hooks --dry-run"          case_install_hooks_dry_run
run_case "install-hooks idempotent"         case_install_hooks_idempotent

echo "test/runtime.sh: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ]
