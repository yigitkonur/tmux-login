package picker

import "strings"

// buildArgs canonicalises the fzf invocation arguments. Order is stable so
// tests can assert against a specific argv shape.
//
// Note: we use --with-nth=2.. to hide the encoded field 1 from display, and
// fzf 0.71+ already restricts the search target to the visible portion when
// --with-nth is set. We do NOT pass --nth=2..; that combination is broken on
// 2-field lines (returns zero matches) — see the regression caught while
// using projects mode after dropping the basename column.
func buildArgs(spec Spec) []string {
	args := []string{
		"--reverse",
		"--no-multi",
		"--height=100%",
		"--header-first",
		"--ansi",
		"--delimiter=\t",
		"--with-nth=2..",
	}
	if spec.PrintQuery {
		args = append(args, "--print-query")
	}
	if spec.Prompt != "" {
		args = append(args, "--prompt="+spec.Prompt)
	}
	if spec.Header != "" {
		args = append(args, "--header="+spec.Header)
	}
	if spec.Query != "" {
		args = append(args, "--query="+spec.Query)
	}
	if len(spec.Expect) > 0 {
		args = append(args, "--expect="+strings.Join(spec.Expect, ","))
	}
	for _, b := range spec.Binds {
		args = append(args, "--bind="+b)
	}
	return args
}

// HeaderFor returns the canonical header line for a given mode plus the
// active hint. Kept here so the wording is consistent across entry points.
func HeaderFor(modeLabel, hint string) string {
	if hint == "" {
		return "[" + modeLabel + "]"
	}
	return "[" + modeLabel + "]   " + hint
}
