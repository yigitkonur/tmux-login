#!/bin/sh
# Sandbox install / uninstall / idempotency / byte-restore tests for
# install.sh and uninstall.sh. POSIX sh.
set -eu

# Resolve repo root from this script's location.
# shellcheck disable=SC1007  # intentional: drop CDPATH for this cd only
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck disable=SC1007
REPO=$(CDPATH='' cd -- "$SCRIPT_DIR/.." && pwd)
BIN="$REPO/bin/tmux-login"

if [ ! -x "$BIN" ]; then
  echo "test/roundtrip.sh: $BIN not built; run 'make build' first" >&2
  exit 1
fi

PASS=0
FAIL=0

run_case() {
  name=$1
  shift
  printf '  case: %-55s ' "$name"
  if (set -e; "$@") >/tmp/tmux-login-roundtrip.log 2>&1; then
    printf 'OK\n'
    PASS=$((PASS + 1))
  else
    printf 'FAIL\n'
    FAIL=$((FAIL + 1))
    echo '----- log -----' >&2
    cat /tmp/tmux-login-roundtrip.log >&2
    echo '---------------' >&2
  fi
}

# Each case sets up its own sandbox; the helpers operate within $tmp.

sandbox_setup() {
  tmp=$(mktemp -d /tmp/tmux-login-rt.XXXXXX)
  HOME=$tmp
  ZDOTDIR=$tmp
  XDG_DATA_HOME=$tmp/.local/share
  XDG_STATE_HOME=$tmp/.local/state
  XDG_CACHE_HOME=$tmp/.cache
  XDG_CONFIG_HOME=$tmp/.config
  export HOME ZDOTDIR XDG_DATA_HOME XDG_STATE_HOME XDG_CACHE_HOME XDG_CONFIG_HOME
}

sandbox_teardown() {
  rm -rf -- "$tmp"
}

install_in_sandbox() {
  sh "$REPO/install.sh" --no-install-deps --bin-from="$BIN" "$@" >/dev/null 2>&1
}

# shellcheck disable=SC2120  # callers may or may not pass args
uninstall_in_sandbox() {
  sh "$REPO/uninstall.sh" "$@" >/dev/null 2>&1
}

# --- 1. fresh install -------------------------------------------------------
case_fresh_install() {
  sandbox_setup
  install_in_sandbox
  [ -x "$XDG_DATA_HOME/tmux-login/bin/tmux-login" ] || { echo "binary missing"; return 1; }
  for f in tmux.conf login-hook.zsh tmux-login.tmux; do
    [ -r "$XDG_DATA_HOME/tmux-login/share/$f" ] || { echo "share/$f missing"; return 1; }
  done
  grep -Fq "tmux-login:hook {{{" "$HOME/.tmux.conf" || { echo "tmux.conf marker missing"; return 1; }
  grep -Fq "tmux-login:hook {{{" "$HOME/.zshrc" || { echo "zshrc marker missing"; return 1; }
  [ -f "$XDG_STATE_HOME/tmux-login/install.json" ] || { echo "install.json missing"; return 1; }
  sandbox_teardown
}

# --- 2. idempotent reinstall ------------------------------------------------
case_idempotent_reinstall() {
  sandbox_setup
  install_in_sandbox
  install_in_sandbox
  count=$(grep -Fc "tmux-login:hook {{{" "$HOME/.tmux.conf")
  [ "$count" = "1" ] || { echo "tmux.conf marker count = $count, want 1"; return 1; }
  count=$(grep -Fc "tmux-login:hook {{{" "$HOME/.zshrc")
  [ "$count" = "1" ] || { echo "zshrc marker count = $count, want 1"; return 1; }
  sandbox_teardown
}

# --- 3. --no-wire -----------------------------------------------------------
case_no_wire() {
  sandbox_setup
  install_in_sandbox --no-wire
  [ -x "$XDG_DATA_HOME/tmux-login/bin/tmux-login" ] || { echo "binary missing"; return 1; }
  [ -f "$HOME/.tmux.conf" ] && { echo "tmux.conf created when --no-wire"; return 1; }
  [ -f "$HOME/.zshrc" ] && { echo ".zshrc created when --no-wire"; return 1; }
  sandbox_teardown
}

# --- 4. --no-tmux-config ----------------------------------------------------
case_no_tmux_config() {
  sandbox_setup
  install_in_sandbox --no-tmux-config
  [ -f "$HOME/.tmux.conf" ] && { echo "tmux.conf created"; return 1; }
  grep -Fq "tmux-login:hook {{{" "$HOME/.zshrc" || { echo "zshrc marker missing"; return 1; }
  sandbox_teardown
}

# --- 5. --no-login-hook -----------------------------------------------------
case_no_login_hook() {
  sandbox_setup
  install_in_sandbox --no-login-hook
  grep -Fq "tmux-login:hook {{{" "$HOME/.tmux.conf" || { echo "tmux.conf marker missing"; return 1; }
  [ -f "$HOME/.zshrc" ] && { echo "zshrc created"; return 1; }
  sandbox_teardown
}

# --- 6. uninstall byte-for-byte restore -------------------------------------
case_uninstall_byte_for_byte() {
  sandbox_setup
  # shellcheck disable=SC2016  # literal $PATH in fixture is intentional
  printf 'alias ll="ls -la"\nexport PATH=$PATH:/foo\n' > "$HOME/.tmux.conf"
  printf 'alias gco="git checkout"\n' > "$HOME/.zshrc"
  cp "$HOME/.tmux.conf" "$tmp/orig-tmux.conf"
  cp "$HOME/.zshrc" "$tmp/orig-zshrc"
  install_in_sandbox
  uninstall_in_sandbox
  cmp -s "$HOME/.tmux.conf" "$tmp/orig-tmux.conf" || { echo "tmux.conf not byte-restored"; diff "$HOME/.tmux.conf" "$tmp/orig-tmux.conf"; return 1; }
  cmp -s "$HOME/.zshrc" "$tmp/orig-zshrc" || { echo "zshrc not byte-restored"; diff "$HOME/.zshrc" "$tmp/orig-zshrc"; return 1; }
  sandbox_teardown
}

# --- 7. uninstall byte-for-byte (no final newline) --------------------------
case_uninstall_byte_for_byte_no_newline() {
  sandbox_setup
  printf 'alias ll="ls -la"' > "$HOME/.tmux.conf"   # no trailing newline
  printf 'alias gco="git checkout"' > "$HOME/.zshrc"
  cp "$HOME/.tmux.conf" "$tmp/orig-tmux.conf"
  cp "$HOME/.zshrc" "$tmp/orig-zshrc"
  install_in_sandbox
  uninstall_in_sandbox
  cmp -s "$HOME/.tmux.conf" "$tmp/orig-tmux.conf" || { echo "tmux.conf not byte-restored (no-newline)"; xxd "$HOME/.tmux.conf"; xxd "$tmp/orig-tmux.conf"; return 1; }
  cmp -s "$HOME/.zshrc" "$tmp/orig-zshrc" || { echo "zshrc not byte-restored (no-newline)"; return 1; }
  sandbox_teardown
}

# --- 8. preserve user content (marker appended at end) ----------------------
case_preserve_existing_user_content() {
  sandbox_setup
  printf 'set -g status-bg blue\nbind a kill-server\n' > "$HOME/.tmux.conf"
  install_in_sandbox
  # First two lines must be the user's content, marker block must be after.
  head -2 "$HOME/.tmux.conf" | grep -Fq 'set -g status-bg blue' || { echo "user content not preserved"; cat "$HOME/.tmux.conf"; return 1; }
  head -2 "$HOME/.tmux.conf" | grep -Fq 'tmux-login:hook' && { echo "marker shouldn't appear in first 2 lines"; return 1; }
  sandbox_teardown
}

# --- run all ----------------------------------------------------------------
echo "test/roundtrip.sh: running cases ..."
run_case "fresh-install"                        case_fresh_install
run_case "idempotent-reinstall"                 case_idempotent_reinstall
run_case "--no-wire"                            case_no_wire
run_case "--no-tmux-config"                     case_no_tmux_config
run_case "--no-login-hook"                      case_no_login_hook
run_case "uninstall-byte-for-byte"              case_uninstall_byte_for_byte
run_case "uninstall-byte-for-byte-no-newline"   case_uninstall_byte_for_byte_no_newline
run_case "preserve-existing-user-content"       case_preserve_existing_user_content

echo "test/roundtrip.sh: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ]
