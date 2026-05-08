# REVIEW.md — tmux-login

What to scrutinize, flag, or block during code review. Pairs with `AGENTS.md` (which covers *how to work*); this file covers *what to verify in diffs*. Rules below are repo-grounded — they encode bugs we've actually shipped and the structural lines we don't want to lose.

## Critical Areas

These changes ALWAYS warrant a careful read. If a PR touches them, the review must verify the listed invariant.

- **`internal/tmux/attach.go`** — the create-or-attach idiom. Verify: `new-session` is invoked WITHOUT `-A` (the `-A -d` combo is broken on tmux 3.4–3.6a — see commit `067159f`). Verify: the path branches on `HasSession` and only emits `new-session -d -s NAME -c DIR` for fresh sessions. Verify: `send-keys` cwd-lock fires only when `wasFresh && spec.Cwd != ""`, never on existing-session attach (would disturb running commands).
- **`internal/tmux/client.go::New()`** — env construction. Verify: NO `LANG=C` / `LC_ALL=C` overrides. Forcing POSIX locale propagates to the tmux server and sanitises every non-ASCII byte (UTF-8, format `\t`, glyphs) to `_`. See commit `af47aa2`.
- **`share/login-hook.zsh`** — the hot path. Verify: every guard before `exec tmux-login login` is parameter-expansion-only (no `$(…)`, no `command -v` outside the eligible-deps check, no `[[ -x … ]]` that hits the filesystem before we know we want to fire). Verify: every nested helper is `unset -f` before the function returns (zsh hoists nested defs to global scope).
- **`internal/install/file.go`** — marker-block read/write. Verify: idempotent `EnsureMarker` (re-running produces exactly one block). Verify: `StripMarker` honors the `# tmux-login:original-no-final-newline` sentinel for byte-for-byte uninstall. Verify: `QuoteShell` escapes single quotes as `'\''` for paths embedded in marker-block source lines.
- **`install.sh`** — binary placement. Verify: rm-then-cp pattern at `$prefix/bin/$BIN_NAME` (macOS Gatekeeper caches a verdict by inode — overwriting in place causes SIGKILL on next exec; commit `cfa5183`). Verify: the ad-hoc `codesign --force --sign -` belt-and-braces still runs on Darwin.
- **`internal/login/login.go`** — picker spec construction. Verify: `--bind=ctrl-k:execute-silent(...)+reload(...)+pos(2)` shape is preserved. Verify: `Spec.Expect` is empty (don't reintroduce `--expect=ctrl-x` — exits the picker on every kill, flicker UX). Verify: `selfBinaryPath()` is shell-quoted via the `'\''`-escape pattern so paths with spaces survive.
- **`internal/login/action.go`** — the kill machinery. Verify: `_action --kill` decodes via `picker.Decode`, calls `tmux has-session` first, only then `tmux kill-session`. Sentinels and zoxide-path rows must be silent no-ops.

## Security

- **No new outbound network in the binary.** The binary is local-only (tmux/fzf/sesh subprocess + filesystem). The installer reaches the network via `git clone`/`brew install`/`curl`, but the binary at runtime must not. Flag any `net/http`, `net.Dial`, or shell-out to `curl`/`wget` from `internal/`.
- **Shell injection in fzf binds.** Any new fzf `--bind=execute*(...)` arg must shell-quote interpolated values via the same single-quote-escape pattern as `selfBinaryPath()`. fzf passes binds to `sh -c`. Argv from user input (session names, paths) into a bind = injection risk.
- **No changes to `~/.ssh/*`, `/etc/ssh/sshd_config`, SSH `ForceCommand`, or `~/.ssh/rc`.** The hook integration point is `~/.zshrc` only. Reject any PR that adds writes outside `~/.tmux.conf`, `~/.zshrc`, `$XDG_DATA_HOME/tmux-login/`, `$XDG_CACHE_HOME/tmux-login/`, `$XDG_STATE_HOME/tmux-login/`, or — at install time — `$HOME/.local/bin/tmux-login` (symlink target).
- **No `.bak` files.** We never full-replace a user file. Any new "back up the user's X" code in `install.sh` deserves scrutiny — we're an append-only marker-block tool.
- **No `sudo` in install.sh except where the user expects it** (apt/dnf/pacman dep installs). Brew runs as the invoking user; never `sudo brew`.

## Conventions

- **Conventional Commits with scope.** `feat(scope): …`, `fix(scope): …`, `tweak(scope): …`, `refactor(scope): …`, `docs(scope): …`, `test(scope): …`. Subject < 50 chars, imperative. Reject `WIP`, `misc`, `updates`, bare `feat:` without a scope.
- **One commit, one purpose.** Reject multi-purpose diffs that mix `fix(picker)` with `tweak(continuum)` in one commit. Each commit must be independently revertable.
- **Comments only for non-obvious WHY.** Reject "// loop through the items" / "// check if it exists" / restating-the-code comments. Comments that explain *why a non-obvious choice was made* (e.g. "we don't use `-A` because tmux 3.4–3.6a triggers attach behavior") are welcome.
- **stdlib only in `internal/`.** No third-party Go imports beyond `stdlib`. The whole point of the binary is a fast static thing; Cobra/Viper/etc. add ~30 ms cold-start. If a PR adds a `go.mod require`, push back hard.
- **Error messages prefixed with `tmux-login:`.** Both Go and shell. Stderr for warnings/errors, stdout for progress.
- **README tone is deliberately casual / lowercase** — don't let a PR "professionalize" it without an explicit ask in the issue.

## Performance

- **Cold-start budget is 120 ms median (1.20× slack = 144 ms).** Asserted by `test/perf.sh`. If a PR adds a new dependency-init path, run `test/perf.sh` and post the median in the PR description. Observed today is ~25 ms.
- **No new third-party Go imports.** Even small ones tax cold-start (init functions). If absolutely required, justify in the PR with measurements before/after.
- **No `tmux list-sessions` in our binary.** That's sesh's job. Re-introducing it duplicates work and re-creates the no-sesh fallback we deleted in v0.3 (~700 LOC removed). Any `c.Output(ctx, "list-sessions", ...)` in a diff outside `internal/sesh/` or `internal/tmux/sessions.go` (which only has `HasSession` + `KillSession`) deserves a "why" question.

## Patterns

- **`internal/tmux` package surface stays small.** Today: `client.go`, `attach.go`, `sessions.go` (HasSession/KillSession only), `server.go` (Version/InsideTmux only). Don't grow this. Anything resembling `windows.go` / `panes.go` / `popup.go` was deleted in v0.3 — those are sesh's or tmux-conf's job.
- **`internal/cache` is MRU-only.** Just `RecordRecentDir` + `RecentDirs`. Don't bring back `attached/<name>` mtime markers, `cwds/<name>`, `.sessions.txt` snapshot, or `GC` — sesh manages session state.
- **Locale: inherit `os.Environ()`.** Reject any `append(env, "LANG=…")` / `"LC_ALL=…"` in `internal/tmux/client.go`. (See `Critical Areas` for why.)
- **`new-session -d` (NEVER `-A`).** See critical areas. The defensive comment in `attach.go` is load-bearing — don't delete it.
- **fzf `--with-nth=2..` alone, no `--nth=2..`.** The pair is broken on 2-field lines in fzf 0.71 (silently zero matches). See commit `fb2eb23`. Don't reintroduce `--nth`.
- **Self-binary path via `os.Executable()`.** Don't switch to `os.Args[0]` (relative path on some kernels) or `exec.LookPath("tmux-login")` alone (misses popups + symlinks).
- **`{{PREFIX}}` sentinel pattern.** `share/tmux.conf` MUST stay path-agnostic in the repo. `install.sh`'s `sed -e "s|{{PREFIX}}|$prefix|g"` substitutes per-machine. If a PR hardcodes a path in `share/tmux.conf`, reject.

## Testing

- **`test/runtime.sh` argv assertions are the contract.** When you change tmux/fzf/sesh argv shape, update the assertions (and the comment explaining why) in the same commit. Don't weaken an assertion to hide a regression.
- **Stub fzf can't simulate fzf bind execution.** Tests for fzf `--bind` flows test the underlying subcommand (`tmux-login _action --kill ...`, `tmux-login _action --list`) directly, not through the stub picker. Don't assert "fzf called my bind" — it can't, and the assertion will lie.
- **Stub-default `SESH_BIN`.** `test/runtime.sh` setup() points `SESH_BIN` at the sesh stub by default (sesh is the only path post-v0.3). Tests don't need to opt in. `MOCK_SESH_LIST` controls list contents.
- **Sandbox via `( export HOME=… ; … )`** sub-shell, not `HOME=… cmd1 | cmd2` — env vars on a pipeline only apply to the leading command. The sandbox pattern in `test/roundtrip.sh` is the canonical form.
- **`test/perf.sh` runs `gdate +%s%N`.** Requires GNU coreutils on macOS (`brew install coreutils`). It's NOT a CI failure if `gdate` is missing — gracefully degrades. But locally, install it.

## Ignore (don't waste review cycles on)

- **gofmt / shellcheck output.** `make check` runs both; CI (when we add it) will gate. Don't nitpick formatting in review.
- **Cosmetic `go vet` output.** Same.
- **`docs(readme): …` typo fixes.** Approve and merge.
- **`tweak(continuum): …` parameter tweaks.** The plan documents reasoning for `@continuum-save-interval = 5`; tweaks should ship with a one-line "why" in the commit body but don't need broad review.
- **Test fixture line shape changes.** As long as `make check` is green, fixture format is implementation detail.

## How to use this file in PR review

For each PR, scan the diff against the section order above:

1. Does it touch a **Critical Area**? Re-read the linked invariant.
2. Does it cross a **Security** boundary (network, ssh files, .bak files, sudo)?
3. Does it follow **Conventions** (commit message, comments, stdlib-only)?
4. Does it preserve **Performance** budgets and avoid forbidden imports?
5. Does it match the **Patterns** the repo already established?
6. Does it update **Testing** assertions when argv shape changes?
7. Anything in **Ignore** is fine — don't bikeshed.

If the PR breaks an invariant, the response should cite the rule by section + bullet (e.g. "Critical Areas → `attach.go` — verify no `-A` in new-session argv"). That keeps review feedback grounded in repo evidence rather than personal taste.
