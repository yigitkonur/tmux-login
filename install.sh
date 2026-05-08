#!/bin/sh
# tmux-login installer — POSIX sh.
# Local:   sh install.sh
# Remote:  curl -fsSL https://raw.githubusercontent.com/yigitkonur/tmux-login/main/install.sh | sh
set -eu

REPO="yigitkonur/tmux-login"
BRANCH="main"
RAW_URL="https://raw.githubusercontent.com/${REPO}/${BRANCH}"

BIN_NAME="tmux-login"
SHARE_FILES="tmux.conf tmux-login.tmux login-hook.zsh"
DEFAULT_PREFIX="${XDG_DATA_HOME:-$HOME/.local/share}/tmux-login"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/tmux-login"

wire_tmux=1
wire_zsh=1
install_deps=1
prefix="$DEFAULT_PREFIX"
bin_from=""

usage() {
  cat <<EOF
Usage: sh install.sh [flags]
   or: curl -fsSL ${RAW_URL}/install.sh | sh

  --no-wire             do not modify ~/.tmux.conf or ~/.zshrc
  --no-tmux-config      do not modify ~/.tmux.conf
  --no-login-hook       do not modify ~/.zshrc
  --no-install-deps     do not auto-install missing tmux / fzf / go
  --prefix=PATH         install under PATH (default: $DEFAULT_PREFIX)
  --bin-from=PATH       use a pre-built binary instead of running 'go build'
  -h, --help            show this help

Environment (read by the binary at runtime):
  TMUX_LOGIN_ROOTS      colon-separated dirs walked by the projects picker
  TMUX_LOGIN_SKIP=1     bypass the SSH-login hook for one shell
  TMUX_LOGIN_PERF=1     enable per-event tracer
  TMUX_LOGIN_PRUNE      extra basenames to skip during the project walk
EOF
}

for arg in "$@"; do
  case "$arg" in
    --no-wire)          wire_tmux=0; wire_zsh=0 ;;
    --no-tmux-config)   wire_tmux=0 ;;
    --no-login-hook)    wire_zsh=0 ;;
    --no-install-deps)  install_deps=0 ;;
    --prefix=*)         prefix="${arg#--prefix=}" ;;
    --bin-from=*)       bin_from="${arg#--bin-from=}" ;;
    -h|--help)          usage; exit 0 ;;
    *)                  printf 'tmux-login: unknown argument: %s\n' "$arg" >&2; usage >&2; exit 2 ;;
  esac
done

info() { printf 'tmux-login: %s\n' "$*"; }
warn() { printf 'tmux-login: %s\n' "$*" >&2; }
die()  { warn "$*"; exit 1; }

# Resolve a relative --prefix to an absolute path. The marker block embeds
# $prefix verbatim, so a relative value would be re-resolved per shell login —
# almost never to the directory the user ran the installer from.
case "$prefix" in
  /*) ;;
  *)
    prefix_orig=$prefix
    prefix_parent=$(dirname -- "$prefix")
    prefix_base=$(basename -- "$prefix")
    mkdir -p -- "$prefix_parent" 2>/dev/null || true
    # shellcheck disable=SC1007  # intentional: drop CDPATH for this cd only
    prefix_abs=$(CDPATH= cd -- "$prefix_parent" 2>/dev/null && pwd) \
      || die "could not resolve --prefix=$prefix_orig to an absolute path"
    prefix="$prefix_abs/$prefix_base"
    ;;
esac

# bootstrap_brew: install Homebrew if missing on macOS.
bootstrap_brew() {
  info "brew not found — bootstrapping Homebrew (may take a few minutes)"
  info "(you may be prompted for your sudo password)"
  if ! (set +e; NONINTERACTIVE=1 /bin/bash -c \
    "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"); then
    warn "Homebrew bootstrap failed — install manually: https://brew.sh"
    return 1
  fi
  if [ -x /opt/homebrew/bin/brew ]; then
    eval "$(/opt/homebrew/bin/brew shellenv)"
  elif [ -x /usr/local/bin/brew ]; then
    eval "$(/usr/local/bin/brew shellenv)"
  fi
  command -v brew >/dev/null 2>&1
}

os="$(uname -s 2>/dev/null || echo unknown)"
case "$os" in
  Darwin)
    if ! command -v brew >/dev/null 2>&1 && [ "$install_deps" -eq 1 ]; then
      bootstrap_brew || true
    fi
    if command -v brew >/dev/null 2>&1; then
      tmux_cmd="brew install tmux"
      fzf_cmd="brew install fzf"
      go_cmd="brew install go"
      sesh_cmd="brew install sesh"
    else
      tmux_cmd=""; fzf_cmd=""; go_cmd=""; sesh_cmd=""
    fi
    ;;
  Linux)
    if command -v apt-get >/dev/null 2>&1; then
      tmux_cmd="sudo apt-get update -qq && sudo apt-get install -y tmux"
      fzf_cmd="sudo apt-get update -qq && sudo apt-get install -y fzf"
      go_cmd="sudo apt-get update -qq && sudo apt-get install -y golang-go"
      sesh_cmd=""  # not in apt; users install via 'go install' or homebrew-on-linux
    elif command -v dnf >/dev/null 2>&1; then
      tmux_cmd="sudo dnf install -y tmux"
      fzf_cmd="sudo dnf install -y fzf"
      go_cmd="sudo dnf install -y golang"
      sesh_cmd=""
    elif command -v pacman >/dev/null 2>&1; then
      tmux_cmd="sudo pacman -S --noconfirm tmux"
      fzf_cmd="sudo pacman -S --noconfirm fzf"
      go_cmd="sudo pacman -S --noconfirm go"
      sesh_cmd="yay -S --noconfirm sesh-bin"  # AUR
    else
      tmux_cmd=""; fzf_cmd=""; go_cmd=""; sesh_cmd=""
    fi
    ;;
  *)
    tmux_cmd=""; fzf_cmd=""; go_cmd=""; sesh_cmd=""
    ;;
esac

ensure_dep() {
  tool=$1; cmd=$2; required=${3:-0}
  command -v "$tool" >/dev/null 2>&1 && return 0
  if [ "$install_deps" -eq 0 ] || [ -z "$cmd" ]; then
    if [ "$required" -eq 1 ]; then
      die "$tool is required — install it manually then re-run."
    fi
    warn "$tool not on PATH — install it manually for full functionality"
    return 1
  fi
  info "$tool not on PATH — auto-installing: $cmd"
  if (set +e; eval "$cmd") && command -v "$tool" >/dev/null 2>&1; then
    info "$tool installed"
    return 0
  fi
  if [ "$required" -eq 1 ]; then
    die "auto-install of $tool failed — install manually then re-run."
  fi
  warn "auto-install of $tool failed — install manually"
  return 1
}

ensure_dep tmux "$tmux_cmd" 1 || true
ensure_dep fzf  "$fzf_cmd"  1 || true
# sesh is REQUIRED as of v0.3 — it provides the session-list source
# (multi-source: tmux + zoxide + sesh.toml + tmuxinator with Nerd Font
# icons) and idempotent attach via `sesh connect`. We deliberately
# don't carry a fallback session-listing path anymore: the codebase is
# meaningfully smaller and there's nothing sesh+brew won't cover.
ensure_dep sesh "$sesh_cmd" 1 || true

# Locate the source tree (clone) or download tarball when run via curl-pipe.
script_dir=""
case "$0" in
  */install.sh|install.sh)
    # shellcheck disable=SC1007
    script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd)" || script_dir=""
    ;;
esac

src_share=""
src_bin="$bin_from"

if [ -n "$script_dir" ] && [ -d "$script_dir/share" ]; then
  src_share="$script_dir/share"
  if [ -z "$src_bin" ] && [ -x "$script_dir/bin/$BIN_NAME" ]; then
    src_bin="$script_dir/bin/$BIN_NAME"
  fi
  info "installing from local clone ($script_dir)"
else
  # curl-pipe path: $0 is "sh" or similar, no local clone. Shallow-clone
  # to a temp dir, build there, throw the clone away. Users who want the
  # source later can `git clone https://github.com/yigitkonur/tmux-login`
  # to wherever they prefer — we don't pollute the filesystem with a
  # permanent source dir.
  command -v git >/dev/null 2>&1 || die "git not on PATH — install git or run install.sh from a clone"
  _clonedir=$(mktemp -d "${TMPDIR:-/tmp}/tmux-login-src.XXXXXX") || die "mktemp failed"
  trap 'rm -rf -- "$_clonedir"' EXIT
  info "fetching source to $_clonedir"
  git clone --depth 1 --quiet "https://github.com/yigitkonur/tmux-login.git" "$_clonedir" \
    || die "git clone failed (check network / proxy)"
  script_dir="$_clonedir"
  src_share="$_clonedir/share"
  info "installing from fresh clone"
fi

# Build binary if not provided.
if [ -z "$src_bin" ]; then
  ensure_dep go "$go_cmd" 0 || true
  if ! command -v go >/dev/null 2>&1; then
    die "go not on PATH and --bin-from not given — install go (or pass --bin-from=PATH)"
  fi
  info "building $BIN_NAME with 'go build'"
  ( cd "$script_dir" && \
    VER=$(git describe --tags --always --dirty 2>/dev/null || echo dev) && \
    SHA=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) && \
    DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    mkdir -p bin && \
    go build -trimpath -ldflags "-s -w -X github.com/yigitkonur/tmux-login/internal/version.Version=$VER -X github.com/yigitkonur/tmux-login/internal/version.Commit=$SHA -X github.com/yigitkonur/tmux-login/internal/version.Date=$DATE" -o "bin/$BIN_NAME" ./cmd/tmux-login )
  src_bin="$script_dir/bin/$BIN_NAME"
fi

# Place binary and share files. Remove the destination first so we get a
# fresh inode — overwriting in place on macOS makes Gatekeeper's cached
# verdict for the old bytes mismatch the new bytes, which presents as a
# SIGKILL on the next exec. rm-then-cp dodges the cache entirely.
mkdir -p -- "$prefix/bin" "$prefix/share"
rm -f -- "$prefix/bin/$BIN_NAME"
cp -- "$src_bin" "$prefix/bin/$BIN_NAME"
chmod +x "$prefix/bin/$BIN_NAME"
# Ad-hoc codesign on macOS so re-installs over a long-lived path don't
# trip the same Gatekeeper cache. Harmless on Linux (codesign absent).
if [ "$os" = "Darwin" ] && command -v codesign >/dev/null 2>&1; then
  codesign --force --sign - "$prefix/bin/$BIN_NAME" 2>/dev/null || true
fi
info "installed binary at $prefix/bin/$BIN_NAME"

# PATH symlink: $prefix/bin is rarely on PATH, but ~/.local/bin is XDG-blessed
# and usually is. Drop a symlink so `tmux-login` works from any shell. If
# the user has neither ~/.local/bin nor /usr/local/bin available, print a
# hint instead.
symlink_target=""
for d in "$HOME/.local/bin" "/usr/local/bin"; do
  if [ -d "$d" ] && [ -w "$d" ]; then
    symlink_target="$d/$BIN_NAME"
    break
  fi
done
if [ -n "$symlink_target" ]; then
  ln -sf "$prefix/bin/$BIN_NAME" "$symlink_target"
  info "symlinked $symlink_target -> $prefix/bin/$BIN_NAME"
else
  warn "no writable \$HOME/.local/bin or /usr/local/bin found — add $prefix/bin to PATH manually"
fi

for f in $SHARE_FILES; do
  if [ ! -f "$src_share/$f" ]; then
    warn "missing source file: $src_share/$f"
    continue
  fi
  # tmux.conf carries a {{PREFIX}} sentinel so the run-shell lines for
  # vendored plugins use absolute paths (tmux's run-shell can't expand
  # shell vars at parse time on all versions). Substitute in-flight.
  case "$f" in
    tmux.conf)
      sed -e "s|{{PREFIX}}|$prefix|g" "$src_share/$f" > "$prefix/share/$f"
      ;;
    *)
      cp -- "$src_share/$f" "$prefix/share/$f"
      ;;
  esac
done
chmod +x "$prefix/share/login-hook.zsh" "$prefix/share/tmux-login.tmux" 2>/dev/null || true
info "installed share files at $prefix/share"

# Vendor tmux-resurrect + tmux-continuum unless they're already present
# under TPM (~/.tmux/plugins/) or already vendored. git-clone shallow.
# share/tmux.conf source-loads from $prefix/share/plugins/, so if the
# user has TPM copies under ~/.tmux/plugins/, our load and theirs may
# both fire — tmux-continuum's multi-server precedence handles it
# (only the first-started server saves/restores). The most likely user
# install (no TPM) gets a single load from our prefix.
plugins_dir="$prefix/share/plugins"
mkdir -p -- "$plugins_dir"
for plugin_pair in 'tmux-resurrect|https://github.com/tmux-plugins/tmux-resurrect' \
                   'tmux-continuum|https://github.com/tmux-plugins/tmux-continuum'; do
  pname=${plugin_pair%%|*}
  purl=${plugin_pair##*|}
  if [ -d "$HOME/.tmux/plugins/$pname" ]; then
    info "$pname already installed via TPM at \$HOME/.tmux/plugins/$pname (skipping vendored copy)"
    # Still place a thin symlink so $prefix/share/plugins/$pname/$pname.tmux
    # resolves; otherwise the run-shell line in share/tmux.conf misses.
    rm -rf -- "${plugins_dir:?}/${pname:?}"
    ln -sf -- "$HOME/.tmux/plugins/$pname" "$plugins_dir/$pname"
    continue
  fi
  if [ -d "$plugins_dir/$pname/.git" ]; then
    # Already vendored by a previous run — cheap pull-update, optional.
    ( cd "$plugins_dir/$pname" && git pull --quiet --ff-only 2>/dev/null ) || true
    info "$pname: already vendored (kept up to date)"
    continue
  fi
  if ! command -v git >/dev/null 2>&1; then
    warn "$pname: git not on PATH — skip vendoring; the persistence layer will be inactive"
    continue
  fi
  info "vendoring $pname → $plugins_dir/$pname"
  rm -rf -- "${plugins_dir:?}/${pname:?}"
  git clone --depth 1 --quiet "$purl" "$plugins_dir/$pname" 2>/dev/null \
    || warn "$pname: clone failed; persistence layer will be inactive"
done

# Wire marker blocks via the Go binary so the awk/regex contract lives in one
# place. install-hooks is idempotent.
if [ "$wire_tmux" -eq 1 ]; then
  "$prefix/bin/$BIN_NAME" install-hooks --tmux --prefix "$prefix"
fi
if [ "$wire_zsh" -eq 1 ]; then
  "$prefix/bin/$BIN_NAME" install-hooks --zsh --prefix "$prefix"
fi

# Diagnostic state file (not load-bearing for uninstall).
mkdir -p -- "$STATE_DIR"
{
  printf '{"version":"%s","prefix":"%s","installed":"%s","wire_tmux":%s,"wire_zsh":%s}\n' \
    "$("$prefix/bin/$BIN_NAME" version | awk '{print $2}')" \
    "$prefix" \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    "$wire_tmux" \
    "$wire_zsh"
} > "$STATE_DIR/install.json"

info "done."
info "Open a new terminal (or 'tmux source-file ~/.tmux.conf') to load the keymap."
info "Verify with: $prefix/bin/$BIN_NAME doctor"
