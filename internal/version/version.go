// Package version reports the build version, set at link time:
//
//	go build -ldflags "-X github.com/lidisoft/horizon/internal/version.Version=v0.1.0 \
//	  -X github.com/lidisoft/horizon/internal/version.Commit=$(git rev-parse --short HEAD)"
package version

// Set by -ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
)

// String returns "version (commit)".
func String() string { return Version + " (" + Commit + ")" }

// MinUpgradeFrom is the oldest release this one migrates from; a database
// on an older release must go through it first. Raised by a release that
// needs it (its notes say so); empty: any.
var MinUpgradeFrom = ""
