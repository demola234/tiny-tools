package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/demola234/tiny-tools/mock_machina/internal/gitfs"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func LoadAt(ctx context.Context, dir, ref string) (*model.Project, error) {
	where := "in the working tree"
	var (
		proj  *model.Project
		probs Problems
		err   error
	)
	if ref == "" {
		proj, probs, err = Load(dir)
	} else {
		where = "at " + ref
		fsys, gitErr := gitfs.At(ctx, dir, ref)
		if gitErr != nil {
			return nil, gitErr
		}
		proj, probs, err = LoadFS(fsys)
	}
	if err != nil {
		return nil, fmt.Errorf("can't read the contract %s: %w", where, err)
	}
	if probs.HasErrors() {
		return nil, fmt.Errorf("the contract %s has problems; fix them first:\n  %s", where, strings.ReplaceAll(probs.String(), "\n", "\n  "))
	}
	return proj, nil
}
