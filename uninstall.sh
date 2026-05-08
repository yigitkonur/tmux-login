#!/bin/sh
# tmux-login uninstaller — POSIX sh.
# Strips marker blocks from ~/.tmux.conf and ~/.zshrc; removes binary,
# share files, state dir, cache dir.
set -eu

DEFAULT_PREFIX="${XDG_DATA_HOME:-$HOME/.local/share}/tmux-login"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/tmux-login"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/tmux-login"
TMUX_CONF="$HOME/.tmux.conf"
ZSHRC="${ZDOTDIR:-$HOME}/.zshrc"

MARK_OPEN="# tmux-login:hook {{{"
MARK_CLOSE="# tmux-login:hook }}}"
MARK_NO_FINAL_NEWLINE="# tmux-login:original-no-final-newline"

prefix="$DEFAULT_PREFIX"
keep_state=0

usage() {
  cat <<EOF
Usage: sh uninstall.sh [flags]

  --prefix=PATH    uninstall from PATH (default: $DEFAULT_PREFIX)
  --keep-state     leave state and cache dirs alone
  -h, --help       show this help
EOF
}

for arg in "$@"; do
  case "$arg" in
    --prefix=*)   prefix="${arg#--prefix=}" ;;
    --keep-state) keep_state=1 ;;
    -h|--help)    usage; exit 0 ;;
    *)            printf 'tmux-login: unknown argument: %s\n' "$arg" >&2; usage >&2; exit 2 ;;
  esac
done

info() { printf 'tmux-login: %s\n' "$*"; }

# strip_marker FILE: removes the marker block from FILE byte-for-byte.
# If MARK_NO_FINAL_NEWLINE was inside the block, the trailing newline is
# trimmed too (preserves the original file's no-newline shape).
strip_marker() {
  file=$1
  [ -f "$file" ] || return 0
  grep -Fq "$MARK_OPEN" "$file" 2>/dev/null || return 0

  tmp="$file.tmux-login.tmp.$$"
  awk -v o="$MARK_OPEN" -v c="$MARK_CLOSE" -v n="$MARK_NO_FINAL_NEWLINE" '
    BEGIN { had_no_newline = 0 }
    index($0, o) > 0  { inblock = 1; next }
    index($0, c) > 0  { inblock = 0; next }
    inblock && index($0, n) > 0 { had_no_newline = 1; next }
    !inblock          { print }
    END { if (had_no_newline) printf "" } # marker: caller trims trailing \n
  ' "$file" > "$tmp"

  # If the block carried the no-final-newline sentinel, remove the last byte
  # if it is a newline.
  if grep -Fq "$MARK_NO_FINAL_NEWLINE" "$file"; then
    # Use perl if available, else dd. perl is on every modern macOS+Linux.
    if command -v perl >/dev/null 2>&1; then
      perl -i -pe 'chomp if eof' "$tmp"
    else
      # Fallback: rewrite without last byte if it's \n.
      last=$(tail -c 1 "$tmp" 2>/dev/null || true)
      if [ "$last" = "" ] || [ "$last" = "
" ]; then
        size=$(wc -c < "$tmp" | tr -d ' ')
        head -c "$((size - 1))" "$tmp" > "$tmp.2" && mv -- "$tmp.2" "$tmp"
      fi
    fi
  fi

  mv -- "$tmp" "$file"
  info "stripped marker block from $file"
}

strip_marker "$TMUX_CONF"
strip_marker "$ZSHRC"

if [ -d "$prefix" ]; then
  rm -f -- "$prefix/bin/tmux-login"
  rm -f -- "$prefix/share/tmux.conf" "$prefix/share/tmux-login.tmux" "$prefix/share/login-hook.zsh"
  rmdir -- "$prefix/bin" 2>/dev/null || true
  rmdir -- "$prefix/share" 2>/dev/null || true
  rmdir -- "$prefix" 2>/dev/null || true
  info "removed $prefix"
fi

if [ "$keep_state" -eq 0 ]; then
  rm -rf -- "$STATE_DIR"
  rm -rf -- "$CACHE_DIR"
  info "removed $STATE_DIR and $CACHE_DIR"
fi

info "done."
