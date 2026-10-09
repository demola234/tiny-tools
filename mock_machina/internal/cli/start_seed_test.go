package cli_test

import (
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const randomProject = `pay:
  route: POST /pay
  mode: random
  states:
    ok: { status: 201 }
    busy: { status: 503 }
    slow: { status: 504 }
`

var seedLine = regexp.MustCompile(`^seed (\d+) \(start with --seed (\d+) to get the same responses again\)$`)

func randomStatuses(t *testing.T, config string, args ...string) (string, []int) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "pay.yaml"), randomProject)
	if config != "" {
		writeFile(t, filepath.Join(dir, "config.yaml"), config)
	}
	lines, stop := startProject(t, dir, args...)
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))
	line := nextLine(t, lines)
	codes := make([]int, 0, 10)
	for range 10 {
		codes = append(codes, post(t, base+"/pay"))
		nextLine(t, lines)
	}
	return line, codes
}

func TestStart_Seed(t *testing.T) {
	t.Parallel()

	line, first := randomStatuses(t, "", "--seed", "7")
	if line != "seed 7 (start with --seed 7 to get the same responses again)" {
		t.Errorf("seed line = %q", line)
	}
	_, again := randomStatuses(t, "", "--seed", "7")
	if !slices.Equal(first, again) {
		t.Errorf("--seed 7 twice gave %v and %v", first, again)
	}

	line, fromConfig := randomStatuses(t, "seed: 7\n")
	if line != "seed 7 (start with --seed 7 to get the same responses again)" || !slices.Equal(first, fromConfig) {
		t.Errorf("seed from config.yaml: %q, %v", line, fromConfig)
	}

	line, _ = randomStatuses(t, "seed: 7\n", "--seed", "8")
	if line != "seed 8 (start with --seed 8 to get the same responses again)" {
		t.Errorf("--seed should win over config.yaml: %q", line)
	}

	line, _ = randomStatuses(t, "")
	m := seedLine.FindStringSubmatch(line)
	if m == nil || m[1] != m[2] {
		t.Fatalf("picked seed line = %q", line)
	}
	if n, _ := strconv.ParseUint(m[1], 10, 64); n == 0 {
		t.Error("picked seed 0")
	}
}

func post(t *testing.T, url string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

func TestStart_FakeDataUsesTheLocale(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	writeFile(t, filepath.Join(dir, "routes", "me.yaml"), "get:\n  route: GET /me\n  states:\n    ok: { body: { phone: \"{{ fake.phone }}\" } }\n")
	writeFile(t, filepath.Join(dir, "config.yaml"), "locale: en_NG\n")
	lines, stop := startProject(t, dir, "--seed", "3")
	defer stop()
	base := servingURL.FindString(nextLine(t, lines))
	nextLine(t, lines)
	if _, body := get(t, base+"/me", ""); !strings.HasPrefix(body, `{"phone":"+234 `) {
		t.Errorf("body = %s, want a Nigerian phone number", body)
	}
}
