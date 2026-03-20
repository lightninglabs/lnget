package build

import "runtime"

// Semantic is the semantic version without commit metadata.
const Semantic = "0.1.0"

// Commit is set during build via ldflags to the git commit hash.
var Commit = "unknown"

// Version returns the current version string including the commit hash.
func Version() string {
	return Semantic + "-" + Commit
}

// VersionInfo returns structured version metadata for JSON output.
func VersionInfo() map[string]string {
	return map[string]string{
		"version":    Semantic,
		"commit":     Commit,
		"go_version": runtime.Version(),
	}
}
