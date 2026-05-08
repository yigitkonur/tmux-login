# AGENTS.md — tmux-login

Instructions for AI agents (Claude Code, Codex, Cursor, etc.) working in this repo. Read before editing.

## What this is

A thin shell on top of upstream tmux-ecosystem tools that gives you tmux as "shell with persistence, not a multiplexer." On interactive SSH login, a zsh hook (sourced from `~/.zshrc`) presents an `fzf` session picker; inside tmux, a single global hotkey (`M-s`) opens the same picker as a `display-popup`. Daily ops never need the prefix key — Alt+letter bindings cover new/close window, jump 1..9, pane focus h/j/k/l, zoom, rename, detach.

We delegate aggressively to upstream:

| Concern | Owned by | Why |
| --- | --- | --- |
| Session listing (multi-source: tmux + zoxide + sesh.toml + tmuxinator, with Nerd Font icons) | [sesh](https://github.com/joshmedeski/sesh) | best-in-class, in homebrew-core, multi-source, configurable |
| Idempotent attach (`sesh connect NAME`) | sesh | handles git-naming, sesh.toml startup_command, zoxide path resolution |
| Auto-save / auto-restore tmux state | [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) + [tmux-continuum](https://github.com/tmux-plugins/tmux-continuum) | vendored under `$prefix/share/plugins/`, sourced from managed tmux.conf |

We own only what those tools don't do:

- **SSH-login hook** — sesh has no equivalent.
- **M-letter no-prefix keymap** — sesh has none.
- **Picker glue** — Ctrl-K kill (in place via fzf `--bind`), `--print-query` rc trap for type-to-create.
- **Type-to-create + dir picker** — sesh's `connect <unknown>` defaults to `$HOME`; we want explicit dir choice.
- **Install hygiene** — marker blocks, byte-restore, `{{PREFIX}}` sentinel substitution, plugin vendoring.

Re-architecture of [yigitkonur/zellij-login](https://github.com/yigitkonur/zellij-login) (UX shape borrowed, code from scratch). Targets tmux 3.4+ (3.6 features opportunistic), fzf 0.55+, sesh 2.x, macOS Darwin + Linux.

## Non-goals (don't propose these)

- **Bash / fish compat for the login hook.** zsh features in `share/login-hook.zsh` are load-bearing: `[[ -o interactive ]]`, parameter-expansion guards, `unset -f` cleanup of nested helpers. Rewriting for portability is a regression.
- **Multi-multiplexer support.** tmux only. zellij users have `zellij-login`.
- **Replacing sesh's job.** Don't add a session-listing fallback path. sesh is required (`brew install sesh` in homebrew-core); the binary fails fast with a clear message if absent. The `internal/tmux/` package handles only kill + the type-to-create attach idiom + doctor queries — never list-sessions.
- **Replacing the user's `~/.tmux.conf` or `~/.zshrc`.** We only append marker blocks. No full-replace, no `.bak` files.
- **TPM as the canonical install path.** TPM-compatible shim (`share/tmux-login.tmux`) is provided, but the canonical path is `install.sh` + the marker block. Plugins are vendored under `$prefix/share/plugins/` (cloned at install time); if user already has TPM copies under `~/.tmux/plugins/`, installer symlinks instead of cloning.
- **Feature growth in the hook.** `share/login-hook.zsh` does one thing: parameter-expansion guards then `exec tmux-login login`. Don't add CLI flags, YAML config, or session templates to the hook.
- **New runtime dependencies.** Allowed: tmux, fzf, sesh, zsh, coreutils, awk, git (for plugin vendoring at install time). Not allowed: gum, broot, yazi, jq, anything else.

## Hard constraints

- **Hot-path discipline (login hook).** `share/login-hook.zsh` runs on every interactive SSH login. Every check before `exec tmux-login login` must be a parameter expansion only — no subshells, no external commands, until we've confirmed the user wants the hook to fire. Order of guards (cheapest first): `[[ -o interactive ]]` → `[[ -n $SSH_TTY ]]` → `[[ -z $TMUX ]]` → `[[ -z $TMUX_LOGIN_SKIP ]]` → IDE exclusions (`VSCODE_IPC_HOOK_CLI`, `CURSOR_SESSION_ID`, `TERM_PROGRAM=vscode`, `TERMINAL_EMULATOR=JetBrains-JediTerm`) → `$TMUX_LOGIN_HOOK_DONE` re-entry guard.
- **Login hook leaks no helpers.** zsh hoists nested function defs to global scope; every helper must `unset -f` before the function returns so the user's interactive shell stays clean.
- **`local` discipline (login hook).** Every variable assigned inside the hook function must be declared `local`. Bare assignments leak into the user's interactive shell after `return`.
- **Idempotent install.** Re-running `install.sh` produces exactly one marker block in each of `~/.tmux.conf` and `~/.zshrc`. Verified by `test/roundtrip.sh`.
- **Byte-for-byte uninstall restore.** `cmp -s` between original and post-uninstall `~/.tmux.conf` / `~/.zshrc` must match, including the no-final-newline case (handled by the `# tmux-login:original-no-final-newline` sentinel).
- **Silent bailout on non-applicable contexts.** Non-interactive shells, IDE remote shells, inside tmux (`$TMUX` set), missing tmux/fzf/sesh — all exit silent (no stderr noise on the hot path) except the explicit `tmux-login: sesh is required` message when sesh is missing on the active code path.
- **Argv contract for type-to-create attach.** `internal/tmux/attach.go` does NOT use `tmux new-session -A -d`. The `-A -d` combo is broken on tmux 3.4–3.6a (`-A` triggers attach behavior and ignores `-d`, then dies on no-tty). Branch explicitly:
  ```
  if !HasSession(NAME) { tmux new-session -d -s NAME -c DIR }
  switch-client -t =NAME    (if $TMUX set)
  exec tmux attach -t =NAME (else; via syscall.Exec)
  ```
  After a fresh create with cwd, `send-keys -t =NAME:.` `cd '<DIR>' && clear` is sent to lock the pane's cwd against shell-startup scripts that drift (cmux relays, terminal shell-integration, plugin chains). Never sent for *existing* sessions — would disturb running commands.
- **fzf `--print-query` rc discipline.** Picker uses `--print-query`, which emits the query on stdout on every exit including Esc (rc=130) and no-match-Enter (rc=1). Capture via the `if raw=$(…); then rc=0; else rc=$?; fi` shape in `internal/picker/picker.go`. rc=130 → bail. rc=1 with non-empty query → type-to-create. Without this trap, Esc-after-typing falls into create-new and spawns an unintended session.
- **Ctrl-K is in-place via fzf bind.** `internal/login/login.go` constructs `--bind=ctrl-k:execute-silent($SELF _action --kill {})+reload($SELF _action --list)+pos(2)`. The picker stays open for chained kills. `--expect=ctrl-x` is GONE — don't reintroduce; on selection, fzf would EXIT the picker (flicker) and re-spawning is sluggish.
- **Hidden underscore subcommand `_action`.** Implemented in `internal/login/action.go`. fzf's `--bind` invokes `tmux-login _action --kill <encoded-line>` and `tmux-login _action --list`. Allocation-light; `--list` re-runs `sesh.List` and re-encodes through `picker.Encode`. `--kill` decodes the line, runs `tmux has-session` to confirm a real session, then `tmux kill-session`. Sentinels and zoxide paths are silent no-ops.
- **`os.Executable()` resolves the bind self-path.** `selfBinaryPath()` in `internal/login/login.go` returns the absolute, shell-quoted path of the running binary so the bind survives popups, symlinks, and paths with spaces.
- **NEVER force `LANG=C` or `LC_ALL=C`** in `internal/tmux/client.go`. Originally we did — it propagates to the tmux SERVER process, and tmux 3.6 sanitises every non-ASCII byte to `_` under POSIX locale. Result: format `\t` becomes `_` (silent session-list parse failure), and every pane the server later spawns mangles UTF-8 (Claude Code's progress bars rendered as `___`, vim box-drawing broken, etc.). Inherit `os.Environ()` unmodified. See commit `af47aa2`.
- **macOS Gatekeeper rm-then-cp pattern.** `install.sh` removes the existing binary at `$prefix/bin/tmux-login` BEFORE `cp` to dodge a stale-bytes verdict in the kernel cache that presents as SIGKILL on next exec. After cp, ad-hoc `codesign --force --sign -` belt-and-braces. See commit `cfa5183`.
- **`{{PREFIX}}` sentinel in `share/tmux.conf`.** Checked-in path-agnostic. `install.sh` does `sed -e "s|{{PREFIX}}|$prefix|g"` per-machine when copying. Plugin `run-shell` lines need absolute paths (tmux's run-shell can't expand shell vars at parse time on all versions).
- **No changes to `~/.ssh/*`, `/etc/ssh/sshd_config`, SSH `ForceCommand`, or `~/.ssh/rc`.** Hook integration point is `~/.zshrc` only.
- **No `.bak` files.** Installer never full-replaces user files; only appends marker blocks.
- **Performance budget.** SSH login → fzf rendered ≤ 120 ms median (asserted by `test/perf.sh`, median × 1.20 slack). Observed: ~25 ms cold-start on darwin/arm64.
- **Go binary discipline.** Built with `go build -trimpath -ldflags '-s -w'`. stdlib `flag` only — no Cobra/Viper (~30 ms cold-start tax). No CGO. Single static binary.

## Testing locally

Never run `install.sh` against your real `~/.tmux.conf` or `~/.zshrc`. Sandbox via `HOME` + `XDG_*` overrides:

```sh
tmp=$(mktemp -d)
( export HOME=$tmp ZDOTDIR=$tmp \
         XDG_DATA_HOME=$tmp/.local/share \
         XDG_CACHE_HOME=$tmp/.cache \
         XDG_STATE_HOME=$tmp/.local/state \
         XDG_CONFIG_HOME=$tmp/.config
  sh install.sh --no-install-deps --bin-from="$(realpath ./bin/tmux-login)" )
# inspect / exercise
sh uninstall.sh
rm -rf $tmp
```

Note: env vars must be exported in a sub-shell or all leading commands in a pipe see them — `HOME=$tmp cat install.sh | sh` only sets `$tmp` for `cat`, not `sh`.

## Before committing

```
make check
```

Runs:
1. `gofmt -l .` (fail if any unformatted)
2. `go vet ./...`
3. `go test ./...` (Go unit tests across all packages)
4. `sh -n` + `shellcheck --shell=sh` on every shell script
5. `zsh -n share/login-hook.zsh`
6. `test/roundtrip.sh` — sandbox install/uninstall/idempotency/byte-restore/no-wire/marker-collision
7. `test/runtime.sh` — PATH-shimmed `tmux` + `fzf` + `sesh` stubs; drives the binary; asserts exact tmux argv shape
8. `test/perf.sh` — `gdate +%s%N` measurements vs the 120 ms SSH-login budget (skip with `SKIP_PERF=1` in CI)

No remote CI yet. Enforcement is client-side: `make hooks` once in your clone wires `.githooks/pre-push` to run `make check` on every `git push`.

## Commit messages

Conventional Commits with a short, descriptive scope — `feat(picker): …`, `fix(installer): …`, `refactor(tmux): …`, `docs(readme): …`, `tweak(continuum): …`. Subject under 50 chars, imperative. No `WIP`, no `misc`, no `updates`. One commit, one purpose.

## File map

| Path | What |
| --- | --- |
| `cmd/tmux-login/main.go` | `flag.FlagSet` subcommand dispatcher: `pick`, `attach`, `login`, `last`, `kill`, `install-hooks`, `doctor`, `version`, `help`, hidden `_action` |
| `internal/version/` | `Version`, `Commit`, `Date` set via `-ldflags -X` at build time |
| `internal/config/` | env-var parsing (`TMUX_LOGIN_{ROOTS,SKIP,PERF,PRUNE}`) + XDG path resolution |
| `internal/sesh/` | sesh CLI wrapper (`Available`, `List`, `Connect`, `Last`); ANSI-strip + Nerd-Font-icon-rune-skip parser; cached `LookPath` |
| `internal/tmux/` | thin `os/exec` wrapper. `client.go` (Cmd/Output/Run, no LC_ALL override), `sessions.go` (HasSession + KillSession ONLY — no list/parse), `server.go` (Version + InsideTmux), `attach.go` (type-to-create idempotent attach + send-keys cwd-lock) |
| `internal/picker/` | fzf orchestration: arg builder, `--print-query` rc parser (rc 0/1/130/2 taxonomy), `Spec.Binds` for `--bind`, encoding (`<action>\x1f<target>\x1f<cwd>\t<display>`) |
| `internal/sources/` | `Item` + `Mode` + `ActionKind` types; `projects.go` walks `$TMUX_LOGIN_ROOTS` for the dir picker (mtime-desc sort, alpha tiebreak, prune list) |
| `internal/cache/` | MRU-only after v0.3: `RecordRecentDir` + `RecentDirs` for the dir picker. No more `attached/<name>`, `cwds/<name>`, `.sessions.txt` snapshot, or `GC` — sesh manages its own state |
| `internal/login/login.go` | SSH-login orchestrator. Single-path post-v0.3: `sx.List` → fzf → on attach `sx.Connect`, on type-to-create our dir picker → `tmux.Attach` (sesh has no `--cwd`). Self-binary path resolution for fzf bind |
| `internal/login/action.go` | Hidden `_action` subcommand; backs the fzf `ctrl-k` bind with `--kill` and `--list` operations |
| `internal/login/dirpick.go` | Second-round fzf for type-to-create: candidates from `internal/sources/projects.go`, query pre-filled with the typed name, `Enter` on no-match auto-mkdirs `<first-root>/<name>` |
| `internal/install/` | Marker-block constants + writers (`HasMarker`, `EnsureMarker`, `StripMarker`, `Diff`, `QuoteShell`); used by `tmux-login install-hooks` |
| `internal/doctor/` | `tmux-login doctor` diagnostic: tmux/fzf/sesh versions, paths, `set-clipboard`, marker-block presence |
| `internal/perf/` | Opt-in tracer (`TMUX_LOGIN_PERF=1` or `$XDG_STATE_HOME/tmux-login/perf.on`) — appends per-event timings to `perf.log` |
| `share/login-hook.zsh` | zsh SSH hook: parameter-expansion guards, then `exec tmux-login login` |
| `share/tmux.conf` | Managed config sourced via marker block. M-keymap (Alt+letter), M-s popup, C-g bypass keytable, copy-mode pbcopy fallback, `@continuum-*` + `@resurrect-*` defaults, status-right `#{continuum_status}` indicator, `run-shell '{{PREFIX}}/share/plugins/...tmux'` (PREFIX substituted at install time) |
| `share/tmux-login.tmux` | TPM-compatible shim — `tmux source-file` the managed `share/tmux.conf` |
| `install.sh` | POSIX-sh installer. Curl-pipe-aware (clone-to-temp-then-build); brew-installs missing tmux/fzf/sesh/go on macOS; vendors plugins under `$prefix/share/plugins/` (or symlinks to `~/.tmux/plugins/` if TPM has them); `sed`-substitutes `{{PREFIX}}` in `share/tmux.conf`; rm-then-cp + ad-hoc codesign; symlinks `~/.local/bin/tmux-login` |
| `uninstall.sh` | POSIX-sh uninstaller. awk-strips marker blocks (honors no-final-newline sentinel); removes binary, share, vendored plugins (preserves TPM copies); cleans `$STATE_DIR` and `$CACHE_DIR` |
| `Makefile` | `build / install / uninstall / check / test / hooks / release` |
| `.githooks/pre-push` | `exec make check` |
| `test/roundtrip.sh` | Sandbox install/uninstall/idempotency/byte-restore/no-wire/marker-collision/preserve-existing-content cases |
| `test/runtime.sh` | PATH-shim tmux + fzf + sesh; drive binary; assert tmux argv. Setup defaults `SESH_BIN` to the stub (sesh is the only path now) |
| `test/perf.sh` | `gdate +%s%N` median-of-5 vs 120 ms budget |
| `test/fixtures/{tmux,fzf,sesh}-stub.sh` | Argv-recording (`$RUN_LOG`) / canned-output (`$FZF_OUTPUTS_DIR/<n>`, `$MOCK_SESH_LIST`, etc.) stubs |

## Style notes

- README tone is deliberately casual / lowercase — don't "professionalize" it without asking.
- Comments in code: only non-obvious WHY. No narration, no history. If a comment explains what the code does, delete it.
- Error messages from shell scripts: prefix with `tmux-login:`. Stderr for warnings and errors, stdout for progress.
- Go: stdlib `flag` only (no Cobra). Errors flow up via `error`; main exits with codes documented in `cmd/tmux-login/main.go`.

## If you break a test

`test/runtime.sh` and `test/roundtrip.sh` are the contract. If a change makes one fail, fix the change or adjust the test — but don't commit with it failing, and don't weaken a test to hide a regression. If the test is wrong, say so in the commit message and explain why.
