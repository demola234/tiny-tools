package buildinfo

import "runtime/debug"

var version string

const develVersion = "(devel)"

const commitLen = 12

type Info struct {
	Version  string
	Commit   string
	Modified bool
}

func (i Info) String() string {
	if i.Commit == "" {
		return i.Version
	}
	if i.Modified {
		return i.Version + " (" + i.Commit + ", modified)"
	}
	return i.Version + " (" + i.Commit + ")"
}

func Read() Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		bi = nil
	}
	return FromBuildInfo(bi, version)
}

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
