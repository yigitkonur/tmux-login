# tmux-login

> tmux as **shell-with-persistence**. one hotkey, one popup, one menu.
> ssh in → session picker before the prompt. inside tmux → `M-s` opens the
> same picker over any pane. no prefix-key gymnastics, ever.

a re-architecture of [yigitkonur/zellij-login](https://github.com/yigitkonur/zellij-login)
for tmux. go binary instead of a 400-line zsh hook on the hot path.
~30 ms cold start. macos + linux.

since v0.3 tmux-login is a thin shell on top of [sesh](https://github.com/joshmedeski/sesh)
(session listing + idempotent attach) and [tmux-resurrect / tmux-continuum](https://github.com/tmux-plugins/tmux-continuum)
(auto-save + auto-restore on server start). we keep only what those
tools don't do: SSH-login hook, M-letter no-prefix keymap, ctrl-k kill
in the picker, type-to-create + dir picker, byte-for-byte install
hygiene. **sesh is required** — `brew install sesh` (homebrew-core);
the binary fails fast with a clear message if absent.

```
                                                  ┌──────────────────────────────────────┐
$ ssh prod                                        │ tmux session > █                     │
.                                                 │ [login]   enter=attach/create  esc=… │
. (zsh hook fires before                          │ ─────────────────────────────────────│
.  the prompt)                                    │ [ skip · plain shell ]               │
.                                                 │ ● admin   (4w, 8m ago)               │
.                                                 │ ● ingest  (2w, 32m ago)              │
.                                                 │ ○ scratch (1w, 3d ago)               │
.                                                 │                                      │
.                                                 │                                      │
                                                  └──────────────────────────────────────┘
```

## why

tmux is great. memorising `prefix + %`, `prefix + "`, `prefix + c`, `prefix + ,` …
is not. tmux-login replaces the prefix-key cognitive tax with one
discoverable popup and a chrome/macos-style alt+letter keymap, while
keeping `C-b` available as an escape hatch for the chord crowd.

it also gives you **session persistence on ssh login** — the killer
feature of the multiplexer experience that almost no plain-tmux setup
delivers out of the box. you ssh into a host, you get a picker for
every existing session plus a "create new" path, you pick, you're in.
disconnect, reconnect, you're right where you left off.

it's a single binary plus a tiny managed `tmux.conf` plus a 30-line zsh
hook. installer is 250 lines of posix sh; uninstall is byte-for-byte.

## what you get

a single global hotkey:

| key | action |
| --- | --- |
| **`M-s`** | open the picker — sessions / projects / recent dirs in one fzf popup |
| **`M-?`** | help / discoverable cheat-sheet |
| **`C-o`** | alias for `M-s` (for vim users with `M-s` clashes) |
| **`C-g`** | bypass mode — let vim/emacs see Alt-keys until pressed again |

a complete window+pane keymap, no prefix:

| key | action | key | action |
| --- | --- | --- | --- |
| `M-t` | new window | `M-n` | split (new pane) |
| `M-w` | close window | `M-q` | close pane |
| `M-1..9` | jump to window N | `M-N` | split vertical |
| `M-]` / `M-[` | next / prev window | `M-h/j/k/l` | focus pane (vim keys) |
| `M-r` | rename window | `M-z` | zoom pane |
| | | `M-d` | detach |

an SSH-login flow that does the right thing every time:

* **type a session name → Enter** → attaches if it exists, else asks for a starting dir
* **type a name that doesn't exist → Enter, Enter** → mkdir under `~/dev/`, attach
* **Esc** at the picker → plain shell, no tmux involvement
* IDE remote shells (vscode, cursor, JetBrains) and non-interactive sessions are auto-skipped via `[[ -o interactive ]]` + IDE-env guards before any work fires

the dir picker is **sorted by mtime** (your last-touched project first),
**filtered against the typed session name** (no retype), and offers a
**type-to-create fast path** — type `testo`, no dir matches, hit Enter,
you're in `~/dev/testo`.

## install

requires **tmux 3.4+**, **fzf 0.55+**, **sesh 2.x** (`brew install sesh`),
**zsh**, and (at install time only) **go 1.22+** for the build. on macOS
the installer brew-installs missing deps automatically.

**one-liner** (recommended):

```sh
curl -fsSL https://raw.githubusercontent.com/yigitkonur/tmux-login/main/install.sh | sh
```

clones the source to a temp dir, builds with `go build`, drops the
binary at `~/.local/share/tmux-login/bin/tmux-login`, symlinks to
`~/.local/bin/tmux-login`, vendors tmux-resurrect + tmux-continuum,
writes the marker blocks. throws the temp clone away when done. open
a new shell (or `tmux source-file ~/.tmux.conf`) and you're in.

**from a clone** (if you want the source for hacking):

```sh
git clone https://github.com/yigitkonur/tmux-login ~/dev/tmux-login
cd ~/dev/tmux-login && make install
```

flags (apply to both forms — pass after `sh -s --` for the curl-pipe form):

| flag | meaning |
| --- | --- |
| `--no-wire` | install binary + share files only; don't touch `~/.tmux.conf` or `~/.zshrc` |
| `--no-tmux-config` | skip the `~/.tmux.conf` marker block (no keymap) |
| `--no-login-hook` | skip the `~/.zshrc` marker block (no SSH-login picker) |
| `--no-install-deps` | don't auto-install missing tmux / fzf via brew/apt/dnf/pacman |
| `--prefix=PATH` | install under PATH (default `$XDG_DATA_HOME/tmux-login`) |
| `--bin-from=PATH` | use a pre-built binary instead of running `go build` |

after install: open a new shell (or `tmux source-file ~/.tmux.conf` if
you're inside tmux). then start a session and press `M-s`.

## uninstall

```sh
cd ~/dev/tmux-login && make uninstall
```

byte-for-byte restores your `~/.tmux.conf` and `~/.zshrc`. removes the
binary, share files, state dir, and cache dir. never touches `~/.ssh/*`
or anything sshd-related.

verified by `test/roundtrip.sh` against a sandbox with seeded user
content + a `cmp -s` assertion (including the no-final-newline edge
case).

## env vars

| var | what |
| --- | --- |
| `TMUX_LOGIN_ROOTS` | colon-separated dirs the project picker walks. default: `$HOME/{dev,code,projects,work,Developer,src,research}` (only the ones that actually exist) |
| `TMUX_LOGIN_SKIP` | set to `1` for one shell to bypass the SSH-login hook |
| `TMUX_LOGIN_PERF` | set to `1` to enable the per-event tracer; logs to `$XDG_STATE_HOME/tmux-login/perf.log`. (alternatively: `touch $XDG_STATE_HOME/tmux-login/perf.on` for sticky enable) |
| `TMUX_LOGIN_PRUNE` | colon-separated extra basenames to skip during the project walk. (default skip-list: `.git`, `node_modules`, `.cache`, `Library`, `.Trash`, `.cargo`, `.rustup`, `.npm`, `dist`, `build`, `target`, `vendor`, `__pycache__`, `.next`, `.nuxt`, `.venv`, `venv`, `.tox`) |
| `TMUX_BIN` | path to the tmux binary (default: PATH lookup) |

## subcommands

```
tmux-login pick [--mode sessions|projects|recent] [--no-popup]
tmux-login attach [--cwd PATH] [--detach] NAME
tmux-login login            # called by the zsh hook on SSH login
tmux-login kill NAME
tmux-login install-hooks --tmux|--zsh [--prefix PATH] [--dry-run]
tmux-login doctor           # diagnostic — versions, paths, marker block presence
tmux-login version
tmux-login help
```

## per-terminal recipes

* **iTerm2 / Ghostty / WezTerm** — zero config. clipboard goes through OSC 52 via `set -g set-clipboard external`.
* **Terminal.app** — OSC 52 not honored. the managed `tmux.conf` ships a `pbcopy` fallback in copy-mode-vi so y/Enter still copies.
* **Alacritty** — set `option_as_alt = "Both"` in your config so Alt-keys reach tmux.
* **Kitty** — set `macos_option_as_alt yes`.

## fish / nu users

the managed `tmux.conf` does **not** touch `default-shell`. if you find
popups behaving oddly with fish (the popup-shell bug from tmux 3.5),
add `set -g default-shell /bin/zsh` in your own `~/.tmux.conf` *below*
the marker block.

## comparison

| | tmux-login | [sesh](https://github.com/joshmedeski/sesh) | [tmux-fzf](https://github.com/sainnhe/tmux-fzf) | [tmux-sessionizer](https://github.com/ThePrimeagen/tmux-sessionizer) | [tmuxinator](https://github.com/tmuxinator/tmuxinator) |
| --- | --- | --- | --- | --- | --- |
| language | go | go | bash | bash | ruby |
| cold-start | ~50 ms | ~80 ms | ~150 ms | ~120 ms | ~400 ms |
| ssh-login picker | **yes** | no | no | no | no |
| sessions + windows + panes + projects + ssh | partial (sessions + projects + recent in v0.1; rest in v0.2) | yes | yes | sessions only | declarative only |
| no-prefix keymap | **shipped** | bring-your-own | bring-your-own | bring-your-own | bring-your-own |
| install footprint | one binary + 3 share files | one binary | bash plugin | 25-line script | ruby gem |
| byte-for-byte uninstall | **yes** | n/a (TPM) | n/a (TPM) | n/a | n/a |

`tmux-login` is the "everything you need to use tmux without learning
its prefix" answer; `sesh` is the slicker daily-driver if you already
have a config you love. they coexist fine on the same machine.

## architecture

```
                    ~/.zshrc                       (interactive shell startup)
                       │
                       │ marker block sources …
                       ▼
            share/login-hook.zsh                   (parameter-expansion guards)
                       │
                       │ exec tmux-login login
                       ▼
                  ┌──────────┐
                  │ go binary │   ──────►  tmux list-sessions / new-session / attach
                  └──────────┘   ──────►  fzf (via display-popup -E from inside tmux)
                       ▲
                       │
                  M-s binding in ~/.tmux.conf
                       │
                       │ marker block sources …
                       ▼
                share/tmux.conf                    (keymap + sane defaults)
```

* go binary, stdlib `flag` only — no Cobra (~30 ms cold-start tax)
* one batched `tmux list-* -F` per call; cache snapshot at `$XDG_CACHE_HOME/tmux-login/.sessions.txt` so the preview pane never re-forks tmux
* xdg paths everywhere; never writes outside `$XDG_{DATA,STATE,CACHE}_HOME` or the user's `~/.tmux.conf` + `~/.zshrc` marker blocks
* zsh hook is parameter-expansion-only on the hot path: ssh-tty / `$TMUX` / IDE / skip-flag / re-entry guards all run before any subshell

full design notes in [`AGENTS.md`](AGENTS.md). research corpus that
informed the design lives in `docs/research/tmux-login/` (popup mechanics,
fzf integration, prior art, performance tradeoffs, version timeline).

## performance

| path | budget | observed median |
| --- | --- | --- |
| ssh login → fzf rendered | ≤ 120 ms | **~18 ms** on darwin/arm64 |
| `M-s` popup → fzf rendered | ≤ 80 ms | (warm tmux server, even faster) |
| dir-picker mode switch (Tab) | ≤ 20 ms | (v0.2) |

`test/perf.sh` enforces. `make check` runs it. budget includes a
1.20× slack so noisy CI doesn't flake.

## development

```sh
make build      # builds bin/tmux-login
make check      # gofmt + go vet + go test + shellcheck + roundtrip + runtime + perf
make test       # alias for check
make hooks      # enables .githooks/pre-push (gates push on make check)
make release    # cross-compile darwin/linux × amd64/arm64 into dist/
make clean      # remove bin/ and dist/
```

stdlib-only go (no third-party deps), posix-sh installer (passes
shellcheck), zsh-only login hook (passes `zsh -n`).

contributions welcome — read [`AGENTS.md`](AGENTS.md) before touching the
hot path. the file lays out the hard constraints (idempotency,
byte-for-byte uninstall, marker-block discipline, hot-path guard
ordering) that the test suite enforces.

## faq

**why not a tpm plugin?** because tpm itself is in maintenance flux and
because installing a binary outright is faster, smaller, and survives
tmux rebuilds without `prefix + I`. tmux-login does ship a TPM-compatible
shim (`share/tmux-login.tmux`) for users who prefer the plugin path.

**why not bash-only?** because the hot path (ssh login → picker
rendered) needs to be ≤ 120 ms with realistic project counts, and
shell loses to a static binary on the order of 5×. zellij-login is
zsh-only and lives at ~80 ms; tmux-login at 18 ms. the `~50 ms` extra
buys typed-data composability across sessions/windows/panes/ssh/projects
sources without a per-source bash script.

**will it work over mosh / et / wezterm-multiplexer / kitty-remote?** yes
— the hook is `$SSH_TTY`-gated, and any tty-bearing remote-shell path
trips that guard. inside the multiplexer, `M-s` works via tmux popup as
normal.

**does it conflict with sesh?** no. they don't bind the same keys by
default. you can use sesh as your daily driver and let tmux-login own
the SSH-login flow.

**why is ~ at the bottom of the dir picker?** because most users live
inside a project root (`~/dev/foo`), not at home. pinning `~` to the
top cost a keystroke every time. it's still in the list.

**what's the C-g bypass keytable?** vim/emacs/htop sometimes have their
own Alt-key bindings. press C-g once to flip into a passthrough mode
where every M-* binding falls through to the inner app. press C-g
again to restore.

## roadmap

* **v0.2** — the rest of the universal menu: windows / panes / ssh /
  kill / help modes, with `Tab` / `Shift-Tab` cycling via fzf's
  `transform-header` + `reload`. zoxide integration. homebrew tap.
  github releases with cross-compiled darwin/linux × amd64/arm64
  artifacts.
* **v0.3** — auto-rename windows from cwd (opt-in via
  `@tmux-login-auto-rename`). tmux 3.6 feature gates (`display-popup -k`,
  modify-popup-inline). linux deb/rpm packages.
* **later** — tmux-resurrect-style state restore. an optional native
  daemon for sub-millisecond mode-switching.

## license

[MIT](LICENSE).

## credits

ux design borrowed from [yigitkonur/zellij-login](https://github.com/yigitkonur/zellij-login).
the idempotent attach idiom (`new-session -A -d -s NAME -c DIR`) and
the type-to-create flow are both from there. tmux-login does not
reuse zellij-login's code — the implementation is from scratch in go,
with the contracts ported deliberately.
