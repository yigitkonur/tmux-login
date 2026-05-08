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
  cp "$FIX/sesh-stub.sh" "$shimdir/sesh"
  chmod +x "$shimdir/tmux" "$shimdir/fzf" "$shimdir/sesh"

  RUN_LOG="$tmp/tmux.argv.log"
  : > "$RUN_LOG"

  # As of v0.3 sesh is required. Default SESH_BIN to the stub so the
  # binary's `sx.Available()` returns true. Tests that want zero items
  # in the picker leave MOCK_SESH_LIST empty/unset (the stub's `list`
  # subcommand emits nothing then).
  SESH_BIN="$shimdir/sesh"
  MOCK_SESH_LIST=""
  export SESH_BIN MOCK_SESH_LIST

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

# Note: existing-session attach is now covered by case_sesh_engine_attach_existing
# (sesh handles attach to existing sessions; the no-sesh tmux-only path is gone).

# --- 1b. attach-fresh-creation sends-keys cd to lock cwd -------------------
case_attach_fresh_sends_cd() {
  setup
  : > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  TMUX_LOGIN_ROOTS="$tmp/dev"
  export MOCK_TMUX_LIST_SESSIONS TMUX_LOGIN_ROOTS

  mkdir -p "$tmp/dev/realdir"

  # fzf #1: rc=1, query "newone". With --expect, emit empty expect-key first.
  printf 'newone\n' > "$FZF_OUTPUTS_DIR/1"
  echo 1 > "$FZF_RC_DIR/1"
  # fzf #2: dir picker (no --expect), pick realdir.
  printf '\nattach\x1fnewone\x1f%s\trealdir\n' "$tmp/dev/realdir" > "$FZF_OUTPUTS_DIR/2"
  echo 0 > "$FZF_RC_DIR/2"

  "$BIN" login

  # Created the session with -c PATH:
  assert_argv_line_has "$RUN_LOG" "new-session -d -s newone -c $tmp/dev/realdir" || return 1
  # AND defensive send-keys cd was issued to the new pane.
  # The stub re-quotes argv when logging, so we check for substrings rather
  # than the exact recorded line.
  if ! grep -F "send-keys" "$RUN_LOG" | grep -F "=newone:." | grep -F "$tmp/dev/realdir" | grep -F "clear" | grep -Fq "Enter"; then
    echo "send-keys cd line missing or malformed in:"
    cat "$RUN_LOG" >&2
    return 1
  fi
  teardown
}

# --- 3. type-to-create ------------------------------------------------------
case_type_to_create() {
  setup
  : > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  export MOCK_TMUX_LIST_SESSIONS

  # fzf #1: --print-query rc=1 with query "newproj"; --expect prepends key line.
  printf 'newproj\n' > "$FZF_OUTPUTS_DIR/1"
  echo 1 > "$FZF_RC_DIR/1"
  # fzf #2: dir picker — user picks /home/u/dev/newproj
  printf '\nattach\x1fnewproj\x1f%s\tnewproj\t~/dev/newproj\n' "$tmp/dev/newproj" > "$FZF_OUTPUTS_DIR/2"
  echo 0 > "$FZF_RC_DIR/2"

  mkdir -p "$tmp/dev/newproj"

  "$BIN" login

  assert_argv_line_has "$RUN_LOG" "new-session -d -s newproj -c $tmp/dev/newproj" || return 1
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

  # fzf #1: --print-query rc=1 with query "testo"; --expect prepends key line.
  printf 'testo\n' > "$FZF_OUTPUTS_DIR/1"
  echo 1 > "$FZF_RC_DIR/1"
  # fzf #2: dir picker (no --expect), also rc=1 with echoed query.
  echo "testo" > "$FZF_OUTPUTS_DIR/2"
  echo 1 > "$FZF_RC_DIR/2"

  # Pre-condition: testo must NOT exist yet.
  [ -d "$tmp/dev/testo" ] && { echo "test setup wrong: $tmp/dev/testo already exists"; return 1; }

  "$BIN" login

  # Post-condition: dir was auto-created, session attached at it.
  [ -d "$tmp/dev/testo" ] || { echo "auto-mkdir failed: $tmp/dev/testo not created"; return 1; }
  assert_argv_line_has "$RUN_LOG" "new-session -d -s testo -c $tmp/dev/testo" || return 1
  teardown
}

# --- 4. dash-prefixed name --------------------------------------------------
case_dash_prefixed_name() {
  setup
  : > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  export MOCK_TMUX_LIST_SESSIONS

  printf -- '-foo\n' > "$FZF_OUTPUTS_DIR/1"
  echo 1 > "$FZF_RC_DIR/1"
  printf '\nattach\x1f-foo\x1f%s\tfoo\t~/dev/foo\n' "$tmp/dev/foo" > "$FZF_OUTPUTS_DIR/2"
  echo 0 > "$FZF_RC_DIR/2"

  mkdir -p "$tmp/dev/foo"

  "$BIN" login

  # The session name "-foo" must reach tmux as a target via -s; if it leaked
  # into a flag slot the stub would swallow it as -f -o -o.
  assert_argv_line_has "$RUN_LOG" "new-session -d -s -foo -c" || return 1
  teardown
}

# --- 5. esc-with-query (rc=130 + non-empty query) --------------------------
case_esc_with_query() {
  setup
  : > "$tmp/sessions"
  MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
  export MOCK_TMUX_LIST_SESSIONS

  # rc=130 + query "newproj". With --expect dropped, fzf emits just the
  # query line; the parser sees: query="newproj", selected="".
  printf 'newproj\n' > "$FZF_OUTPUTS_DIR/1"
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
  assert_argv_line_has "$RUN_LOG" "new-session -d -s myproj -c /tmp/x" || return 1
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

# --- 12. ctrl-x kill from picker, then enter on remaining session ----------
# Note: ctrl-x kill against a real session is covered by
# case_sesh_engine_ctrlx_kill (sesh-engine path is the only one now).

# --- 13. _action --kill on the [skip] sentinel is a no-op ------------------
# (the kill is now invoked via fzf bind execute-silent — we test the
# subcommand directly since the stub fzf can't simulate bind execution.)
case_action_kill_on_sentinel() {
  setup

  # The [skip] sentinel encoded line: action="skip", target empty.
  "$BIN" _action --kill 'skip\x1f\x1f\t[ skip · plain shell ]' 2>&1

  if grep -F "kill-session" "$RUN_LOG" >/dev/null 2>&1; then
    echo "_action --kill on a sentinel must be a no-op"
    cat "$RUN_LOG" >&2
    return 1
  fi
  teardown
}

# ─── sesh-engine cases ──────────────────────────────────────────────────────
# When sesh is on PATH, the binary uses sesh's multi-source list and routes
# attaches through `sesh connect`. ctrl-x kill still goes to tmux directly.

# --- 14. sesh-engine: pick existing session, attach via sesh connect --------
case_sesh_engine_attach_existing() {
  setup
  SESH_BIN="$shimdir/sesh"
  MOCK_SESH_LIST="$tmp/sesh.list"
  printf '\x1b[34m \x1b[39m alpha\n\x1b[36m \x1b[39m ~/dev/proj\n' > "$MOCK_SESH_LIST"
  export SESH_BIN MOCK_SESH_LIST

  # fzf #1: user picks the alpha row. With --expect dropped, fzf emits
  # the query line then the selected line — no leading expect-key line.
  printf '\nattach\x1falpha\x1f\t\x1b[34m \x1b[39m alpha\n' > "$FZF_OUTPUTS_DIR/1"
  echo 0 > "$FZF_RC_DIR/1"

  "$BIN" login

  # Must call sesh list first, then sesh connect alpha.
  assert_argv_line_has "$RUN_LOG" "sesh list --icons" || return 1
  assert_argv_line_has "$RUN_LOG" "sesh connect alpha" || return 1
  # MUST NOT fall back to tmux new-session.
  if grep -F "tmux new-session" "$RUN_LOG" >/dev/null 2>&1; then
    echo "regression: tmux new-session called when sesh-engine active"
    cat "$RUN_LOG" >&2
    return 1
  fi
  teardown
}

# --- 15. sesh-engine: type-to-create still uses our dir picker -------------
case_sesh_engine_type_to_create() {
  setup
  SESH_BIN="$shimdir/sesh"
  MOCK_SESH_LIST="$tmp/sesh.list"
  : > "$MOCK_SESH_LIST"   # sesh sees zero items
  TMUX_LOGIN_ROOTS="$tmp/dev"
  export SESH_BIN MOCK_SESH_LIST TMUX_LOGIN_ROOTS

  mkdir -p "$tmp/dev/realdir"

  # fzf #1: --print-query rc=1 (no match), query "newone".
  printf 'newone\n' > "$FZF_OUTPUTS_DIR/1"
  echo 1 > "$FZF_RC_DIR/1"
  # fzf #2: dir picker (no --expect), pick realdir.
  printf '\nattach\x1fnewone\x1f%s\trealdir\n' "$tmp/dev/realdir" > "$FZF_OUTPUTS_DIR/2"
  echo 0 > "$FZF_RC_DIR/2"

  "$BIN" login

  # Type-to-create flow goes through our tmux path (NOT sesh) because we
  # need to honour the explicit dir choice from the dir picker.
  assert_argv_line_has "$RUN_LOG" "tmux new-session -d -s newone -c $tmp/dev/realdir" || return 1
  if grep -F "sesh connect" "$RUN_LOG" >/dev/null 2>&1; then
    echo "regression: sesh connect called for type-to-create (should use our tmux path)"
    cat "$RUN_LOG" >&2
    return 1
  fi
  teardown
}

# --- 16. _action --kill on a real session calls tmux kill-session ----------
# (replaces the older case_sesh_engine_ctrlx_kill that drove a stubbed fzf
# --expect=ctrl-x flow; the bind path runs the kill via execute-silent in
# fzf itself, which our shell-stub fzf can't simulate. Test the subcommand.)
case_action_kill_real_session() {
  setup
  MOCK_TMUX_HAS_SESSIONS="alpha"
  export MOCK_TMUX_HAS_SESSIONS

  # Encoded picker line for an existing session 'alpha'.
  "$BIN" _action --kill "$(printf 'attach\x1falpha\x1f\tdisplay-text')"

  assert_argv_line_has "$RUN_LOG" "tmux has-session -t =alpha" || return 1
  assert_argv_line_has "$RUN_LOG" "tmux kill-session -t =alpha" || return 1
  if grep -F "sesh" "$RUN_LOG" >/dev/null 2>&1; then
    echo "regression: sesh called during _action --kill"
    cat "$RUN_LOG" >&2
    return 1
  fi
  teardown
}

# --- 16b. _action --kill on a zoxide path is a no-op (no session exists) ---
case_action_kill_zoxide_path() {
  setup
  # Empty MOCK_TMUX_HAS_SESSIONS → has-session always returns 1.
  export MOCK_TMUX_HAS_SESSIONS=""

  # Encoded picker line for a zoxide path (target = "~/dev/proj").
  "$BIN" _action --kill "$(printf 'attach\x1f~/dev/proj\x1f\tdisplay-text')"

  assert_argv_line_has "$RUN_LOG" "tmux has-session" || return 1
  if grep -F "kill-session" "$RUN_LOG" >/dev/null 2>&1; then
    echo "kill-session should NOT fire when has-session returns 1"
    cat "$RUN_LOG" >&2
    return 1
  fi
  teardown
}

# --- 16c. _action --list emits encoded picker lines from sesh.List ---------
case_action_list_emits_encoded_lines() {
  setup
  SESH_BIN="$shimdir/sesh"
  MOCK_SESH_LIST="$tmp/sesh.list"
  printf '\x1b[34m \x1b[39m alpha\n\x1b[36m \x1b[39m ~/dev/proj\n' > "$MOCK_SESH_LIST"
  export SESH_BIN MOCK_SESH_LIST

  out=$("$BIN" _action --list 2>&1)

  # First line should be the [skip] sentinel.
  echo "$out" | head -1 | grep -Fq '[ skip · plain shell ]' \
    || { echo "_action --list missing skip sentinel"; echo "$out"; return 1; }
  # Each item line is `<encoded>\t<display>`; check the encoded prefix
  # contains the target.
  echo "$out" | grep -Fq 'alpha' || { echo "_action --list missing 'alpha'"; echo "$out"; return 1; }
  # shellcheck disable=SC2088  # literal tilde is what sesh emits; we don't want $HOME expansion
  echo "$out" | grep -Fq '~/dev/proj' || { echo "_action --list missing '~/dev/proj'"; echo "$out"; return 1; }
  teardown
}

# Note: there is no "sesh unavailable fallback" anymore (v0.3). If sesh
# isn't on PATH, the binary prints a clear stderr message and exits.

# --- 18. tmux-login last subcommand routes to sesh last --------------------
case_last_subcommand() {
  setup
  SESH_BIN="$shimdir/sesh"
  export SESH_BIN

  "$BIN" last

  assert_argv_line_has "$RUN_LOG" "sesh last" || return 1
  teardown
}

echo "test/runtime.sh: running cases ..."
run_case "fresh-attach sends cd lock"       case_attach_fresh_sends_cd
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
run_case "_action --kill on sentinel"       case_action_kill_on_sentinel
run_case "_action --kill real session"      case_action_kill_real_session
run_case "_action --kill zoxide path"       case_action_kill_zoxide_path
run_case "_action --list emits lines"       case_action_list_emits_encoded_lines
run_case "sesh-engine attach existing"      case_sesh_engine_attach_existing
run_case "sesh-engine type-to-create"       case_sesh_engine_type_to_create
run_case "tmux-login last subcommand"       case_last_subcommand

echo "test/runtime.sh: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ]
