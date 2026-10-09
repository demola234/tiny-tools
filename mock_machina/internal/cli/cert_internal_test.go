package cli

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInstallSteps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos      string
		simulator bool
		want      [][]string
	}{
		{"darwin", true, [][]string{
			{"security", "add-trusted-cert", "-r", "trustRoot", "-k", "/home/login.keychain-db", "/ca/rootCA.pem"},
			{"xcrun", "simctl", "keychain", "booted", "add-root-cert", "/ca/rootCA.pem"},
		}},
		{"darwin", false, [][]string{
			{"security", "add-trusted-cert", "-r", "trustRoot", "-k", "/home/login.keychain-db", "/ca/rootCA.pem"},
		}},
		{"linux", false, [][]string{
			{"sudo", "cp", "/ca/rootCA.pem", "/usr/local/share/ca-certificates/mockmachina.crt"},
			{"sudo", "update-ca-certificates"},
		}},
		{"windows", false, [][]string{
			{"certutil", "-user", "-addstore", "Root", "/ca/rootCA.pem"},
		}},
	}
	for _, tc := range tests {
		got := installSteps(tc.goos, "/ca/rootCA.pem", "/home/login.keychain-db", tc.simulator)
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("%s (simulator %v) (-want +got):\n%s", tc.goos, tc.simulator, diff)
		}
	}
}
