#!/usr/bin/env bash
# tmux-login TPM shim. If you use TPM (tmux-plugin-manager), add to ~/.tmux.conf:
#
#     set -g @plugin 'yigitkonur/tmux-login'
#
# and TPM will source this file. Outside TPM, the install.sh script writes a
# marker block that source-files share/tmux.conf directly — both paths end up
# loading the same managed config.
set -eu

_tl_dir=$(cd -- "$(dirname -- "$0")" && pwd)
_tl_conf="$_tl_dir/tmux.conf"

if [ -r "$_tl_conf" ]; then
  tmux source-file "$_tl_conf"
fi
