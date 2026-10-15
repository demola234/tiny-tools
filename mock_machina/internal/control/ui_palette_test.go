package control_test

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var paletteLink = regexp.MustCompile(`<a href="([^"]+)"`)

func paletteHrefs(t *testing.T, f fixture, q string) []string {
	t.Helper()
	rec := f.raw(t, "GET", "/__mockmachina/ui/palette?q="+q, "", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("palette %q = %d %q", q, rec.Code, rec.Header().Get("Content-Type"))
	}
	matches := paletteLink.FindAllStringSubmatch(rec.Body.String(), -1)
	if len(matches) == 0 {
		return nil
	}
	hrefs := make([]string, 0, len(matches))
	for _, m := range matches {
		hrefs = append(hrefs, m[1])
	}
	return hrefs
}

func TestUIPalette(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, "", map[string]string{"routes/users.yaml": uiRoutesYAML, "routes/cart.yaml": cartYAML})
	tests := []struct {
		q    string
		want []string
	}{
		{"", []string{
			"/__mockmachina/ui/", "/__mockmachina/ui/routes", "/__mockmachina/ui/traffic", "/__mockmachina/ui/data",
			"/__mockmachina/ui/scenarios", "/__mockmachina/ui/coverage",
			"/__mockmachina/ui/routes/cart.items", "/__mockmachina/ui/routes/users.get", "/__mockmachina/ui/routes/users.list",
		}},
		{"USERS/{", []string{"/__mockmachina/ui/routes/users.get"}},
		{"traf", []string{"/__mockmachina/ui/traffic"}},
		{"cart", []string{"/__mockmachina/ui/routes/cart.items"}},
		{"zzz", nil},
	}
	for _, tt := range tests {
		if diff := cmp.Diff(tt.want, paletteHrefs(t, f, tt.q)); diff != "" {
			t.Errorf("palette %q (-want +got):\n%s", tt.q, diff)
		}
	}
}
