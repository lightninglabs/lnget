package build

// Commit is set during build via ldflags to the git commit hash.
var Commit = "unknown"

// Version returns the current version string including the commit hash.
func Version() string {
	return "0.1.0-" + Commit
}
