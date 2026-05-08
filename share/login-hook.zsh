#!/usr/bin/env zsh
# tmux-login zsh login hook — sourced from ~/.zshrc inside the marker block.
#
# Hot-path discipline (AGENTS.md): every guard before exec must be a
# parameter expansion only. No subshells, no external commands, until we've
# confirmed the user wants the hook to fire. Order: cheapest checks first.

_tmux_login_hook() {
  [[ -o interactive ]]                            || return 0
  [[ -n $SSH_TTY ]]                               || return 0
  [[ -z $TMUX ]]                                  || return 0
  [[ -z $TMUX_LOGIN_SKIP ]]                       || return 0
  [[ -z $VSCODE_IPC_HOOK_CLI ]]                   || return 0
  [[ -z $CURSOR_SESSION_ID ]]                     || return 0
  [[ $TERM_PROGRAM != vscode ]]                   || return 0
  [[ $TERMINAL_EMULATOR != JetBrains-JediTerm ]]  || return 0
  [[ -z $TMUX_LOGIN_HOOK_DONE ]]                  || return 0
  export TMUX_LOGIN_HOOK_DONE=1

  # Locate the binary. Prefer $TMUX_LOGIN_BIN (set by the installer when --prefix
  # is non-default) for path determinism; fall back to PATH lookup.
  local _tmux_login_bin
  if [[ -n $TMUX_LOGIN_BIN && -x $TMUX_LOGIN_BIN ]]; then
    _tmux_login_bin=$TMUX_LOGIN_BIN
  else
    _tmux_login_bin=$(command -v tmux-login 2>/dev/null)
  fi
  [[ -n $_tmux_login_bin ]] || return 0

  # Warp Terminal wraps interactive shells in a per-tab ZDOTDIR (warptmp.XXXXXX)
  # whose .zshrc chain-sources the real ~/.zshrc; inside a multiplexer that
  # chain can deadlock waiting on terminal-integration handshakes the multiplexer
  # doesn't pass through. Drop the override so the tmux session picks up
  # $HOME/.zshrc directly. Same fix zellij-login ships.
  [[ $ZDOTDIR == */warptmp.* ]] && unset ZDOTDIR

  "$_tmux_login_bin" login
}

_tmux_login_hook || true
unset -f _tmux_login_hook 2>/dev/null || true
