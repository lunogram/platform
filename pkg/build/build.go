package build

import (
	"runtime/debug"
)

// name contains the name of the compiled binaries. This variable is
// populated while building the service through LD flags.
var name string

// ServiceName returns the name of the service. The value <unknown> is returned
// if the name is not set.
func ServiceName() string {
	if name == "" {
		return "<unknown>"
	}

	return name
}

// version contains the version of the compiled binaries. This variable is
// populated while building the service through LD flags.
var version string

// Version returns the version of the service. The value <unknown> is returned
// if the version is not set.
func Version() string {
	if version == "" {
		return "<unknown>"
	}

	return version
}

// commit contains the commit hash of the compiled binaries. This variable is
// set to commit hash at runtime from the build info
var commit string

func init() {
	// Read the commit hash from the build info
	info, _ := debug.ReadBuildInfo()
	if info != nil {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				commit = setting.Value
			}
		}
	}
}

// Commit returns the commit hash of the service. The value <unknown> is
// returned if the commit hash is not set.
func Commit() string {
	if commit == "" {
		return "<unknown>"
	}

	return commit
}
