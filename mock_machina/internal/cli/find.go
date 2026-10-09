package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

var ErrNoProject = errors.New("no .mockmachina folder")

const dirFlagUsage = "project folder (default: the nearest .mockmachina here or above)"

func FindProject(start string) (string, error) {
	dir := start
	for {
		candidate := filepath.Join(dir, config.DirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%w in %s or any folder above it (create one, or pass --dir)", ErrNoProject, start)
		}
		dir = parent
	}
}

func projectDir(cmd *cobra.Command, flagValue string) (string, error) {
	if cmd.Flags().Changed("dir") {
		return flagValue, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindProject(wd)
}

func nearestProject() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return FindProject(wd)
}
