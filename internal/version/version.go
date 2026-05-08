// Package version exposes build-time identity. Values are populated via
// -ldflags -X at build time (see Makefile); zero-values are valid for tests.
package version

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
