package buildinfo_test

import (
	"runtime/debug"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/buildinfo"
)

func TestFromBuildInfo(t *testing.T) {
	t.Parallel()

	const rev = "fe81932323512865fa46ca967a317bb43373788b"
	withVCS := func(version string, modified string) *debug.BuildInfo {
		return &debug.BuildInfo{
			Main: debug.Module{Version: version},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: rev},
				{Key: "vcs.modified", Value: modified},
			},
		}
	}

	tests := []struct {
		name     string
		bi       *debug.BuildInfo
		override string
		want     buildinfo.Info
	}{
		{
			name: "no build info",
			bi:   nil,
			want: buildinfo.Info{Version: "(devel)"},
		},
		{
			name: "empty main version",
			bi:   &debug.BuildInfo{},
			want: buildinfo.Info{Version: "(devel)"},
		},
		{
			name: "tagged module version without vcs",
			bi:   &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}},
			want: buildinfo.Info{Version: "v0.1.0"},
		},
		{
			name: "devel build with clean vcs",
			bi:   withVCS("(devel)", "false"),
			want: buildinfo.Info{Version: "(devel)", Commit: "fe8193232351"},
		},
		{
			name: "devel build with local changes",
			bi:   withVCS("(devel)", "true"),
			want: buildinfo.Info{Version: "(devel)", Commit: "fe8193232351", Modified: true},
		},
		{
			name:     "override wins over module version",
			bi:       withVCS("v0.1.0", "false"),
			override: "v0.2.0",
			want:     buildinfo.Info{Version: "v0.2.0", Commit: "fe8193232351"},
		},
		{
			name: "short revision kept as is",
			bi: &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123"},
			}},
			want: buildinfo.Info{Version: "(devel)", Commit: "abc123"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := buildinfo.FromBuildInfo(tc.bi, tc.override)
			if got != tc.want {
				t.Errorf("FromBuildInfo() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestInfo_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info buildinfo.Info
		want string
	}{
		{"version only", buildinfo.Info{Version: "v0.1.0"}, "v0.1.0"},
		{"with commit", buildinfo.Info{Version: "v0.1.0", Commit: "a1b2c3d4e5f6"}, "v0.1.0 (a1b2c3d4e5f6)"},
		{"modified", buildinfo.Info{Version: "(devel)", Commit: "fe8193232351", Modified: true}, "(devel) (fe8193232351, modified)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.info.String(); got != tc.want {
				t.Errorf("Info.String() = %q, want %q", got, tc.want)
			}
		})
	}
}
