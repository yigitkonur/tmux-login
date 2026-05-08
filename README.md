# tmux-login

tmux as shell-with-persistence. one hotkey, one popup, one menu — sessions, windows, panes, projects, ssh, recent dirs, kill / rename / detach. ssh into a host and you get a session picker before the prompt. no prefix-key gymnastics.

re-architecture of [zellij-login](https://github.com/yigitkonur/zellij-login) for tmux. go binary instead of zsh hot path. ~50–80 ms cold start.

## what you get

- **`M-s`** opens an fzf popup with sessions, windows, panes, projects (under `~/dev` etc.), ssh hosts from `~/.ssh/config`, and recent dirs. tab cycles modes. type to filter. enter attaches/switches.
- **alt+letter for everything** — chrome/macos style:
  - `M-t` new window · `M-w` close window · `M-1..9` jump to window N · `M-]/M-[` next/prev window · `M-r` rename
  - `M-n` split (new pane) · `M-q` close pane · `M-h/j/k/l` focus pane · `M-z` zoom · `M-d` detach
  - `M-?` help · `C-o` alias for `M-s`
  - `C-b` left as default prefix for power users — never required for daily ops
  - `C-g` bypass mode (lets vim/inner-app see Alt-keys until pressed again)
- **ssh login** drops you into a session picker. `[skip]` = plain shell. type a name = create.
- **idempotent attach** — every `tmux new-session -A -d` is safe to re-run.
- **byte-for-byte uninstall.** `~/.tmux.conf` and `~/.zshrc` go back to exactly what they were.

## install

requires tmux 3.4+, fzf 0.55+, zsh, and go 1.22+ (only at install time).

```sh
git clone https://github.com/yigitkonur/tmux-login ~/dev/tmux-login
cd ~/dev/tmux-login
make install
```

or one-liner (when the github release is up):

```sh
curl -fsSL https://raw.githubusercontent.com/yigitkonur/tmux-login/main/install.sh | sh
```

flags:
- `--no-wire` — install binary + share files only; don't touch `~/.tmux.conf` or `~/.zshrc`
- `--no-tmux-config` — install everything except the `~/.tmux.conf` marker block
- `--no-login-hook` — install everything except the `~/.zshrc` marker block (no ssh-login auto-popup)
- `--no-install-deps` — skip homebrew bootstrap and `brew install tmux fzf`
- `--prefix=PATH` — install under PATH (default `$XDG_DATA_HOME/tmux-login`)
- `--bin-from=PATH` — use a pre-built binary instead of running `go build`

## uninstall

```sh
cd ~/dev/tmux-login && make uninstall
# or
curl -fsSL https://raw.githubusercontent.com/yigitkonur/tmux-login/main/uninstall.sh | sh
```

## env vars

| var | what |
| --- | --- |
| `TMUX_LOGIN_ROOTS` | colon-separated dirs the project picker walks (default: `$HOME/{dev,code,projects,work,Developer,src,research}`) |
| `TMUX_LOGIN_SKIP` | set to `1` for one shell to bypass the ssh-login hook |
| `TMUX_LOGIN_PERF` | set to `1` to enable the perf tracer (or `touch $XDG_STATE_HOME/tmux-login/perf.on` for sticky enable) |
| `TMUX_LOGIN_PRUNE` | colon-separated extra basenames to skip during project walk (in addition to `.git node_modules .cache Library .Trash .cargo .rustup .npm dist build target vendor __pycache__ .next .nuxt .venv venv .tox`) |

## subcommands

```
tmux-login pick [--mode sessions|windows|panes|projects|ssh|recent|kill|help] [--no-popup]
tmux-login attach <name> [--cwd PATH] [--detach]
tmux-login login            # run automatically by the zsh hook on ssh login
tmux-login kill <session>
tmux-login install-hooks --tmux|--zsh [--prefix PATH] [--dry-run]
tmux-login doctor           # diagnostic
tmux-login version
tmux-login help [<subcommand>]
```

## per-terminal recipes

**iTerm2 / Ghostty / WezTerm:** zero config, OSC 52 clipboard works through `set-clipboard external`.

**Terminal.app:** OSC 52 not honored. The managed `tmux.conf` ships a `pbcopy` fallback in copy-mode-vi.

**Alacritty / Kitty:** make sure `Alt` is sent as escape sequences (option +Esc on macOS Alacritty: `option_as_alt = "Both"`; Kitty: `macos_option_as_alt yes`).

## fish / nu users

the managed `tmux.conf` does NOT touch `default-shell`. if you find popups behaving oddly with fish, set `set -g default-shell /bin/zsh` in your own `~/.tmux.conf` *below* the marker block.

## performance budget

| path | budget |
| --- | --- |
| ssh login → fzf rendered | ≤ 120 ms median |
| `M-s` popup → fzf rendered | ≤ 80 ms median |
| mode switch (tab → reload) | ≤ 20 ms median |

`test/perf.sh` enforces. `make check` runs it.

## license

MIT.
