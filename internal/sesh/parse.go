package sesh

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// parseList walks the output of `sesh list --icons`. Each line looks like:
//
//	\x1b[34m  <NF-icon>\x1b[39m <name>      (color 34 = tmux session)
//	\x1b[36m  <NF-icon>\x1b[39m <path>      (color 36 = zoxide entry)
//	\x1b[33m  <NF-icon>\x1b[39m <name>      (config / tmuxinator entries; color varies)
//
// Display keeps the raw line for fzf to render with --ansi (so the user
// sees coloured icons). Target is the bare name or path that
// `sesh connect` accepts.
func parseList(b []byte) []Item {
	out := make([]Item, 0)
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		target, ok := extractTarget(string(line))
		if !ok {
			continue
		}
		out = append(out, Item{
			Display: string(line),
			Target:  target,
		})
	}
	return out
}

// extractTarget strips the ANSI escape sequences and the leading icon glyph
// from a sesh-list line, returning the bare name/path. Returns ok=false on
// malformed input (we then skip the line rather than passing junk to
// `sesh connect`).
//
// Implementation: strip-ANSI, trim whitespace, drop the first rune (the
// icon), trim again, take the rest. The icon is always exactly one rune
// regardless of how many bytes it encodes.
func extractTarget(line string) (string, bool) {
	clean := stripANSI(line)
	clean = strings.TrimLeft(clean, " \t")
	if clean == "" {
		return "", false
	}
	// Drop one rune (the Nerd Font glyph).
	_, sz := utf8.DecodeRuneInString(clean)
	if sz == 0 {
		return "", false
	}
	rest := clean[sz:]
	rest = strings.TrimLeft(rest, " \t")
	rest = strings.TrimRight(rest, " \t\r")
	if rest == "" {
		return "", false
	}
	return rest, true
}

// stripANSI removes CSI escape sequences (the only kind sesh emits).
// Pattern: \x1b[ ... letter. We don't need a full regex engine; a tiny
// state machine is faster and stdlib-only.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip until a final byte (0x40-0x7e — letter or related).
			j := i + 2
			for j < len(s) {
				c := s[j]
				if c >= 0x40 && c <= 0x7e {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
