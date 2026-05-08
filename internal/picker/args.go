package picker

import "strings"

// buildArgs canonicalises the fzf invocation arguments. Order is stable so
// tests can assert against a specific argv shape.
func buildArgs(spec Spec) []string {
	args := []string{
		"--reverse",
		"--no-multi",
		"--height=100%",
		"--header-first",
		"--ansi",
		"--delimiter=\t",
		"--with-nth=2..",
		"--nth=2..",
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
