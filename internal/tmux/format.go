package tmux

// Format strings for `tmux list-* -F`. Tab-delimited so we can split on '\t'
// without escaping. Order matters — see the corresponding parsers below.

const (
	SessionFormat = "#{session_name}\t#{session_attached}\t#{session_last_attached}\t#{session_path}\t#{session_windows}"
	WindowFormat  = "#{session_name}\t#{window_index}\t#{window_id}\t#{window_name}\t#{window_active}\t#{pane_current_command}"
	PaneFormat    = "#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_id}\t#{pane_current_command}\t#{pane_current_path}"
)
