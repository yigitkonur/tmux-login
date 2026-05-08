package install

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// HasMarker reports whether the file at path already contains MarkOpen.
// Missing file → false, no error (idempotent install treats this as "needs add").
func HasMarker(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if strings.Contains(sc.Text(), MarkOpen) {
			return true, nil
		}
	}
	return false, sc.Err()
}

// EnsureMarker appends a marker block to path if not already present. Block
// content is the lines passed in (without the open/close markers — those are
// added). When the existing file has no trailing newline, a sentinel comment
// is inserted inside the block so Strip() can preserve the original byte
// shape on uninstall.
//
// Returns true if the file was modified, false if it already had the marker.
func EnsureMarker(path string, blockLines []string) (bool, error) {
	had, err := HasMarker(path)
	if err != nil {
		return false, err
	}
	if had {
		return false, nil
	}

	// Read existing content so we can detect the no-final-newline case.
	var orig []byte
	if data, err := os.ReadFile(path); err == nil {
		orig = data
	} else if !os.IsNotExist(err) {
		return false, err
	}

	var buf bytes.Buffer
	addedSeparator := false
	if len(orig) > 0 {
		buf.Write(orig)
		if orig[len(orig)-1] != '\n' {
			buf.WriteByte('\n')
			addedSeparator = true
		}
	}
	buf.WriteString(MarkOpen)
	buf.WriteByte('\n')
	if addedSeparator {
		buf.WriteString(MarkNoFinalNewline)
		buf.WriteByte('\n')
	}
	for _, line := range blockLines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	buf.WriteString(MarkClose)
	buf.WriteByte('\n')

	// Atomic write: temp + rename.
	tmp := path + ".tmux-login.tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}

// StripMarker removes any marker-delimited block from path. If the block
// contained MarkNoFinalNewline, the trailing newline is also trimmed so the
// uninstall is byte-for-byte against the original. Missing file is a no-op.
//
// Returns true if a block was removed.
func StripMarker(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var (
		out               bytes.Buffer
		inBlock           bool
		stripped          bool
		hadNoFinalNewline bool
	)
	r := bufio.NewReader(bytes.NewReader(data))
	for {
		line, err := r.ReadString('\n')
		if line == "" && err == io.EOF {
			break
		}
		trimmed := strings.TrimRight(line, "\n")
		switch {
		case strings.Contains(trimmed, MarkOpen):
			inBlock = true
			stripped = true
		case strings.Contains(trimmed, MarkClose):
			inBlock = false
		case inBlock:
			if strings.Contains(trimmed, MarkNoFinalNewline) {
				hadNoFinalNewline = true
			}
			// drop other in-block lines
		default:
			out.WriteString(line)
		}
		if err == io.EOF {
			break
		}
	}

	if !stripped {
		return false, nil
	}

	final := out.Bytes()
	if hadNoFinalNewline && len(final) > 0 && final[len(final)-1] == '\n' {
		final = final[:len(final)-1]
	}

	tmp := path + ".tmux-login.tmp"
	if err := os.WriteFile(tmp, final, 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}

// QuoteShell returns a single-quoted POSIX-shell rendering of s. Embedded
// single quotes become '\” (close, escaped, reopen). Mirrors zellij-login's
// quote_shell() helper.
func QuoteShell(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Diff returns the textual diff that EnsureMarker would write, without
// touching the file. Useful for `--dry-run`.
func Diff(path string, blockLines []string) (string, error) {
	had, err := HasMarker(path)
	if err != nil {
		return "", err
	}
	if had {
		return fmt.Sprintf("# %s already has tmux-login marker; no changes\n", path), nil
	}
	var buf strings.Builder
	fmt.Fprintf(&buf, "# %s — would append:\n", path)
	buf.WriteString(MarkOpen + "\n")
	for _, l := range blockLines {
		buf.WriteString(l + "\n")
	}
	buf.WriteString(MarkClose + "\n")
	return buf.String(), nil
}
