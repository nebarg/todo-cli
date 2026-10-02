// Package buildinfo reports the version a command was built at.
package buildinfo

import "runtime/debug"

// Version is the module version recorded in the binary: a tag such as
// v1.2.0, a pseudo-version, or either with +dirty for a tree with uncommitted
// changes. It is "(devel)" when the build recorded none, as go run does, or
// when build information is unavailable.
func Version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}
