# AGENTS.md — tmux-login

Instructions for AI agents (Claude Code, Codex, Cursor, etc.) working in this repo. Read before editing.

## What this is

A Go binary + tiny `.tmux` shim + POSIX-sh installer that gives you tmux as "shell with persistence, not a multiplexer." On interactive SSH login, a zsh hook (sourced from `.zshrc`) presents an `fzf` session picker and either attaches or creates a tmux session in a chosen project directory. Inside tmux, a single global hotkey (`M-s`) opens the same picker as a `display-popup`. Daily ops never need the prefix key — Alt+letter bindings cover new/close window, jump 1..9, pane focus h/j/k/l, zoom, rename, detach.

Re-architecture of [yigitkonur/zellij-login](https://github.com/yigitkonur/zellij-login). Borrows the UX shape, not the code. Targets tmux 3.4+ (3.6 features opportunistic), fzf 0.55+, macOS Darwin + Linux.

## Non-goals

- **Bash / fish compat for the login hook.** zsh features are load-bearing in `share/login-hook.zsh`: `${(@f)…}`, `(s.:.)`, `local -a`, `[[ -o interactive ]]`. Rewriting for portability is a regression.
- **Multi-multiplexer support.** tmux only. zellij users have `zellij-login`; screen users are on their own.
- **Feature growth.** Hook does one thing: pick + attach. Configuration is env-var only (`TMUX_LOGIN_ROOTS`, `TMUX_LOGIN_SKIP`). Don't add CLI flags to the hook, YAML config, or session templates.
- **New runtime dependencies.** Allowed: tmux, fzf, zsh, coreutils, awk. Optional: zoxide. Not allowed: gum, broot, yazi, jq, anything else.
- **TPM as the install path.** Provided as a convenience shim; the canonical install is `install.sh` + the marker block.
- **Replacing `~/.tmux.conf`.** Installer NEVER full-replaces the user's tmux config — only appends a marker block that `source-file`s our managed config.

## Hard constraints

- **Hot-path discipline (login hook).** `share/login-hook.zsh` runs on every interactive SSH login. Every check before `exec tmux-login login` must be a parameter expansion only (no subshells, no external commands) — until we've confirmed the user wants the hook to fire. Order of guards (cheapest first): `[[ -o interactive ]]` → `[[ -n $SSH_TTY ]]` → `[[ -z $TMUX ]]` → IDE exclusions → `[[ -z $TMUX_LOGIN_SKIP ]]` → `$TMUX_LOGIN_HOOK_DONE` re-entry guard.
- **Login hook leaks no helpers.** zsh hoists nested function defs to global scope; every helper must `unset -f` before return so the user's interactive shell stays clean.
- **`local` discipline (login hook).** Every variable assigned inside a hook function must be declared `local`. Bare assignments leak into the user's interactive shell after `return`.
- **Idempotent install.** Re-running `install.sh` produces exactly one marker block in each of `~/.tmux.conf` and `~/.zshrc`. Verified by `test/roundtrip.sh`.
- **Byte-for-byte uninstall restore.** `cmp -s` between original and post-uninstall `.tmux.conf` / `.zshrc` must match, including the no-final-newline case (handled by `ZSH_MARK_NO_FINAL_NEWLINE` sentinel).
- **Silent bailout on non-applicable contexts.** Non-interactive shells, IDE remote shells (`VSCODE_IPC_HOOK_CLI`, `CURSOR_SESSION_ID`, `JetBrains-JediTerm`, `TERM_PROGRAM=vscode`), inside tmux (`$TMUX`), missing tmux/fzf binaries — all exit silent (no stderr noise on the hot path).
- **Argv contract for attach.** `tmux new-session -A -d -s -- NAME -c DIR` (with the `--` separator and `-A -d` for idempotent create-or-noop), then `tmux switch-client -t -- NAME` if `$TMUX` set, else `tmux attach -t -- NAME`. Asserted by `test/runtime.sh`.
- **fzf `--print-query` rc discipline.** Picker uses `--print-query`, which emits the query on stdout on every exit including Esc (rc=130) and no-match-Enter (rc=1). Capture via `if raw=$(…); then rc=0; else rc=$?; fi`. rc=130 → bail. rc=1 with non-empty query → type-to-create. Without this trap, Esc-after-typing falls into create-new and spawns an unintended session.
- **Hidden underscore subcommands.** `_emit-items`, `_emit-header`, `_action` are fzf-bind backends. Allocation-light. No JSON, no struct tags, raw `bufio.Writer` to stdout. Every Tab keypress invokes one.
- **No changes to `~/.ssh/*`, `/etc/ssh/sshd_config`, SSH `ForceCommand`, or `~/.ssh/rc`.** Hook integration point is `.zshrc` only.
- **No `.bak` files.** Unlike zellij-login, we never full-replace any user file — only append marker blocks. So `install.sh` MUST NEVER create `.tmux.conf.bak` or `.zshrc.bak`.
- **Performance budget.** SSH login → fzf rendered: ≤ 120 ms median. M-s popup → fzf rendered: ≤ 80 ms median. Mode switch (Tab → reload): ≤ 20 ms median. Asserted by `test/perf.sh` (median × 1.20 slack).
- **Go binary discipline.** Built with `-trimpath -ldflags '-s -w'`. stdlib `flag` only — no Cobra/Viper (~30 ms cold-start cost). No CGO. Single static binary.

## Testing locally

Never run `install.sh` against your real `.tmux.conf` or `.zshrc`. Use the sandbox pattern from `test/roundtrip.sh`:

```sh
tmp=$(mktemp -d)
export HOME=$tmp \
       ZDOTDIR=$tmp \
       XDG_DATA_HOME=$tmp/.local/share \
       XDG_CACHE_HOME=$tmp/.cache \
       XDG_STATE_HOME=$tmp/.local/state \
       XDG_CONFIG_HOME=$tmp/.config
sh install.sh --no-install-deps --bin-from=$(realpath ./bin/tmux-login)
# inspect / exercise
sh uninstall.sh
rm -rf $tmp
```

## Before committing

```
make check
```

Runs `gofmt -l && go vet`, `go test ./...`, `sh -n` + `shellcheck` on every shell script, `zsh -n` on `share/login-hook.zsh`, `test/roundtrip.sh`, `test/runtime.sh`, and `test/perf.sh`.

No remote CI yet. Enforcement is client-side: run `make hooks` once in your clone and `make check` runs automatically on every `git push` (`.githooks/pre-push`). A failing check aborts the push.

## Commit messages

Conventional Commits with a short, descriptive scope — `feat(picker): …`, `fix(installer): …`, `refactor(tmux): …`, `docs(readme): …`. Subject under 50 chars, imperative. No `WIP`, no `misc`, no `updates`. One commit, one purpose.

## File map

| Path | What |
| --- | --- |
| `cmd/tmux-login/main.go` | `flag.FlagSet` subcommand dispatcher (`pick`, `attach`, `login`, `kill`, `install-hooks`, `doctor`, `version`, `help`, hidden `_emit-items`/`_emit-header`/`_action`) |
| `internal/version/` | `Version`, `Commit`, `Date` set via `-ldflags -X` at build time |
| `internal/config/` | `TMUX_LOGIN_{ROOTS,SKIP,PERF,PRUNE}` env-var parsing + XDG path resolution |
| `internal/tmux/` | `os/exec`-backed tmux client; format helpers; sessions/windows/panes/attach/popup/server |
| `internal/cache/` | XDG cache layout (`attached/<name>` mtime markers, `cwds/<name>`, `recent_dirs` MRU 50, `.sessions.txt` snapshot) |
| `internal/sources/` | `Item` + `Source` interface; per-mode source packages (`tmux_sessions`, `projects`, `recent`) |
| `internal/picker/` | fzf orchestration: arg builder, `--print-query` rc parser, mode bindings, mode rotation |
| `internal/login/` | SSH-login orchestrator (analog of zellij-login's `_zellij_login_hook` body, in Go) |
| `internal/doctor/` | `tmux-login doctor` diagnostic (versions, paths, state, marker-block presence) |
| `internal/install/` | Marker-block constants + writers for `~/.tmux.conf` and `~/.zshrc` (used by `tmux-login install-hooks`) |
| `internal/perf/` | Opt-in tracer (`TMUX_LOGIN_PERF=1` or sentinel file `$XDG_STATE_HOME/tmux-login/perf.on`) |
| `share/tmux.conf` | Managed tmux config: keymap (Alt+letter parity), sane defaults, popup hotkey, copy-mode + pbcopy fallback, C-g bypass keytable. Sourced via marker block. |
| `share/tmux-login.tmux` | TPM-compatible shim — locates the binary, sources `share/tmux.conf`, exits |
| `share/login-hook.zsh` | zsh SSH hook: parameter-expansion guards, then `exec tmux-login login` |
| `install.sh` | POSIX-sh installer. Builds the Go binary (or extracts a release artifact), copies `share/`, writes marker blocks to `~/.tmux.conf` and `~/.zshrc`, writes `$STATE_DIR/install.json` for diagnostics. |
| `uninstall.sh` | POSIX-sh uninstaller. Strips marker blocks via awk; removes binary, `share/`, state dir, cache dir. |
| `Makefile` | `build / install / uninstall / check / test / hooks / release` |
| `.githooks/pre-push` | `exec make check` |
| `test/roundtrip.sh` | Sandbox install/idempotency/uninstall/byte-restore/marker-collision/no-wire/curl-pipe-detection cases |
| `test/runtime.sh` | PATH-shimmed tmux + fzf; drives the binary; asserts exact tmux argv shape |
| `test/perf.sh` | `gdate +%s%N` measurements vs budget; fails on regression |
| `test/fixtures/{tmux,fzf}-stub.sh` | Argv-recording / output-canned stubs |

## Style notes

- README tone is deliberately casual / lowercase — don't "professionalize" it without asking.
- Comments in code: only non-obvious WHY. No narration, no history. If a comment explains what the code does, delete it.
- Error messages from shell scripts: prefix with `tmux-login:`. Stderr for warnings and errors, stdout for progress.
- Go: stdlib `flag` only (no Cobra). Errors flow up via `error`; main exits with codes documented in `cmd/tmux-login/main.go`.

## If you break a test

`test/runtime.sh` and `test/roundtrip.sh` are the contract. If a change makes one fail, fix the change or adjust the test — but don't commit with it failing, and don't weaken the test to hide a regression. If the test is wrong, say so in the commit message and explain why.
