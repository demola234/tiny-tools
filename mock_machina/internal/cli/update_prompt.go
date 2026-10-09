package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/demola234/tiny-tools/mock_machina/internal/update"
)

const (
	checkEvery   = 24 * time.Hour
	checkTimeout = 3 * time.Second
	barCells     = 24
	megabyte     = 1 << 20
)

var (
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	accent        = lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA")).Bold(true)
	success       = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true)
	faint         = lipgloss.NewStyle().Faint(true)
	noCheckArgs   = []string{"update", "mcp", "help", "completion", "__complete", "__completeNoDesc", "--version", "-v", "--help", "-h"}
)

type progressBar struct {
	out   io.Writer
	label string
	frame int
}

func (b *progressBar) update(done, total int64) {
	spin := accent.Render(spinnerFrames[b.frame%len(spinnerFrames)])
	b.frame++
	size := fmt.Sprintf("%.1f MB", float64(done)/megabyte)
	bar := ""
	if total > 0 {
		filled := int(done * barCells / total)
		bar = " " + accent.Render(strings.Repeat("▰", filled)) + faint.Render(strings.Repeat("▱", barCells-filled))
		size = fmt.Sprintf("%.1f / %.1f MB", float64(done)/megabyte, float64(total)/megabyte)
	}
	_, _ = fmt.Fprintf(b.out, "\r\x1b[K%s %s%s %s", spin, b.label, bar, faint.Render(size))
}

func (b *progressBar) finish(msg string) {
	_, _ = fmt.Fprintf(b.out, "\r\x1b[K%s %s\n", success.Render("✓"), msg)
}

type checkState struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
	Skipped string    `json:"skipped,omitempty"`
}

type updateCheck struct {
	current string
	state   string
	now     time.Time
	in      io.Reader
	out     io.Writer
	src     update.Source
	apply   func(context.Context, string) error
}

func (c updateCheck) run(ctx context.Context) {
	if !strings.HasPrefix(c.current, "v") {
		return
	}
	st := c.load()
	if st.Latest == "" || c.now.Sub(st.Checked) >= checkEvery {
		lookup, cancel := context.WithTimeout(ctx, checkTimeout)
		latest, err := c.src.Latest(lookup)
		cancel()
		if err != nil {
			return
		}
		st.Checked, st.Latest = c.now, latest
		c.save(st)
	}
	if !update.Newer(c.current, st.Latest) || st.Latest == st.Skipped {
		return
	}
	_, _ = fmt.Fprintf(c.out, "%s mockmachina %s is out (you have %s). Update now? [Y/n] ", accent.Render("↑"), st.Latest, c.current)
	switch strings.ToLower(strings.TrimSpace(readLine(c.in))) {
	case "", "y", "yes":
		if err := c.apply(ctx, st.Latest); err != nil {
			_, _ = fmt.Fprintf(c.out, "update failed: %v\n", err)
		}
	default:
		st.Skipped = st.Latest
		c.save(st)
		_, _ = fmt.Fprintln(c.out, faint.Render("skipped; run mockmachina update when you're ready"))
	}
}

func readLine(r io.Reader) string {
	var b strings.Builder
	one := make([]byte, 1)
	for {
		n, err := r.Read(one)
		if n == 1 && one[0] != '\n' {
			b.WriteByte(one[0])
		}
		if err != nil || (n == 1 && one[0] == '\n') {
			return b.String()
		}
	}
}

func (c updateCheck) load() checkState {
	var st checkState
	if data, err := os.ReadFile(c.state); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	return st
}

func (c updateCheck) save(st checkState) {
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(c.state), 0o755) == nil {
		_ = os.WriteFile(c.state, data, 0o644)
	}
}

func wantsUpdateCheck(args []string, getenv func(string) string) bool {
	if getenv("MOCKMACHINA_NO_UPDATE_CHECK") != "" || getenv("CI") != "" {
		return false
	}
	return !slices.ContainsFunc(args, func(a string) bool { return slices.Contains(noCheckArgs, a) })
}

func checkStatePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "mockmachina", "update.json")
}
