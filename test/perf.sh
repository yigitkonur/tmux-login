#!/bin/sh
# Performance tests — measures cold-start of the binary against budgets.
# Uses gdate +%s%N (coreutils on macOS); falls back to gracefully skipping
# if neither gdate nor `date +%s%N` is available.
set -eu

# shellcheck disable=SC1007
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck disable=SC1007
REPO=$(CDPATH='' cd -- "$SCRIPT_DIR/.." && pwd)
BIN="$REPO/bin/tmux-login"
FIX="$SCRIPT_DIR/fixtures"

if [ ! -x "$BIN" ]; then
  echo "test/perf.sh: $BIN not built" >&2
  exit 1
fi

# Pick a sub-millisecond timer.
ns_now() {
  if command -v gdate >/dev/null 2>&1; then
    gdate +%s%N
  elif date +%s%N 2>/dev/null | grep -Eq '^[0-9]+$'; then
    date +%s%N
  else
    # Fallback: 1-second resolution. Budget assertions become 1-second-safe.
    printf '%s000000000' "$(date +%s)"
  fi
}

# medians: read 5 numbers from stdin, output the median.
median() {
  sort -n | awk '{ a[NR]=$1 } END { if (NR==0) {print 0; exit}; print a[int((NR+1)/2)] }'
}

# Build a sandbox tmux+fzf shim that returns instantly so we measure our
# own cost, not tmux's.
tmp=$(mktemp -d /tmp/tmux-login-perf.XXXXXX)
trap 'rm -rf -- "$tmp"' EXIT

shimdir="$tmp/shim"
mkdir -p -- "$shimdir"
cp "$FIX/tmux-stub.sh" "$shimdir/tmux"
cp "$FIX/fzf-stub.sh"  "$shimdir/fzf"
chmod +x "$shimdir/tmux" "$shimdir/fzf"

# A synthetic 5-session list so list-sessions returns nontrivial data.
cat > "$tmp/sessions" <<EOF
alpha	1	1700000001	/home/u/dev/alpha	2
beta	0	1700000002	/home/u/dev/beta	3
gamma	1	1700000003	/home/u/dev/gamma	1
delta	0	1700000004	/home/u/dev/delta	5
epsilon	0	1700000005	/home/u/dev/epsilon	2
EOF

HOME=$tmp
ZDOTDIR=$tmp
XDG_CACHE_HOME=$tmp/.cache
XDG_STATE_HOME=$tmp/.local/state
XDG_DATA_HOME=$tmp/.local/share
XDG_CONFIG_HOME=$tmp/.config
TMUX=""
TMUX_LOGIN_SKIP=""
PATH="$shimdir:$PATH"
MOCK_TMUX_LIST_SESSIONS="$tmp/sessions"
export HOME ZDOTDIR XDG_CACHE_HOME XDG_STATE_HOME XDG_DATA_HOME XDG_CONFIG_HOME \
       TMUX TMUX_LOGIN_SKIP PATH MOCK_TMUX_LIST_SESSIONS

# fzf returns "Esc on empty" so the binary exits without further work after
# the picker — gives us picker-end-to-end timing.
FZF_OUTPUTS_DIR="$tmp/fzf-out"
FZF_RC_DIR="$tmp/fzf-rc"
FZF_STATE_DIR="$tmp/fzf-state"
mkdir -p -- "$FZF_OUTPUTS_DIR" "$FZF_RC_DIR" "$FZF_STATE_DIR"
for i in 1 2 3 4 5 6 7 8 9 10; do
  printf '\n' > "$FZF_OUTPUTS_DIR/$i"  # empty query, no selection
  echo 130 > "$FZF_RC_DIR/$i"           # Esc
done
export FZF_OUTPUTS_DIR FZF_RC_DIR FZF_STATE_DIR

measure() {
  cmd=$1
  iters=$2
  for _ in $(seq 1 "$iters"); do
    rm -f "$FZF_STATE_DIR/counter"
    t0=$(ns_now)
    eval "$cmd" >/dev/null 2>&1 || true
    t1=$(ns_now)
    echo "scale=2; ($t1 - $t0) / 1000000" | bc
  done
}

# v0.1 budget (per the plan, with 1.20× slack):
#   SSH login (cold) → ≤ 120ms × 1.20 = 144ms
# v0.2 will measure popup + mode switch separately.
BUDGET_LOGIN_MS=144

echo "test/perf.sh: measuring (5 iterations) ..."
login_med=$(measure "$BIN login" 5 | median)

printf '  login (cold) median = %s ms (budget: %s ms)\n' "$login_med" "$BUDGET_LOGIN_MS"

# Compare via awk (median may have a decimal).
if awk -v m="$login_med" -v b="$BUDGET_LOGIN_MS" 'BEGIN { exit !(m+0 <= b+0) }'; then
  echo "test/perf.sh: PASS"
  exit 0
else
  echo "test/perf.sh: FAIL — login median ${login_med}ms exceeds budget ${BUDGET_LOGIN_MS}ms"
  exit 1
fi
