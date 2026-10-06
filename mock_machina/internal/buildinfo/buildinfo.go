package buildinfo

import "runtime/debug"

// version overrides the module version for release builds:
//
//	go build -ldflags "-X github.com/demola234/tiny-tools/mock_machina/internal/buildinfo.version=v0.1.0"
var version string

// develVersion is what Go reports for a build that isn't a tagged module version.
const develVersion = "(devel)"

// commitLen is how many characters of the revision are shown, matching Go's
// pseudo-versions.
const commitLen = 12

// Info describes one build of mockmachina.
type Info struct {
	Version  string // "v0.1.0", a pseudo-version, or "(devel)"
	Commit   string // shortened VCS revision; empty when unknown
	Modified bool   // built from a tree with uncommitted changes
}

// String renders the info for --version, e.g. "v0.1.0 (a1b2c3d4e5f6)" or
// "(devel) (fe8193232351, modified)".
func (i Info) String() string {
	if i.Commit == "" {
		return i.Version
	}
	if i.Modified {
		return i.Version + " (" + i.Commit + ", modified)"
	}
	return i.Version + " (" + i.Commit + ")"
}

// Read returns the info for the running binary.
func Read() Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		bi = nil
	}
	return FromBuildInfo(bi, version)
}

// FromBuildInfo builds Info from Go's embedded build information. A non-empty
// override replaces the module version. bi may be nil.
func FromBuildInfo(bi *debug.BuildInfo, override string) Info {
	info := Info{Version: develVersion}
	if bi != nil {
		if bi.Main.Version != "" {
			info.Version = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = shorten(s.Value)
			case "vcs.modified":
				info.Modified = s.Value == "true"
			}
		}
	}
	if override != "" {
		info.Version = override
	}
	return info
}

func shorten(rev string) string {
	if len(rev) > commitLen {
		return rev[:commitLen]
	}
	return rev
}
