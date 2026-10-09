package gitfs

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing/fstest"
)

func At(ctx context.Context, projectDir, ref string) (fs.FS, error) {
	if strings.HasPrefix(ref, "-") {
		return nil, fmt.Errorf("%q isn't a git ref", ref)
	}
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	project, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	top, err := git(ctx, project, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%s isn't inside a git repository", projectDir)
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, project)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)

	if _, err := git(ctx, root, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
		return nil, fmt.Errorf("unknown git ref %q", ref)
	}
	listed, err := git(ctx, root, "ls-tree", "--name-only", ref, "--", rel)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(listed)) == 0 {
		return fstest.MapFS{}, nil
	}
	archive, err := git(ctx, root, "archive", "--format=tar", ref, "--", rel)
	if err != nil {
		return nil, err
	}
	return untar(archive, rel+"/")
}

func untar(archive []byte, prefix string) (fs.FS, error) {
	files := fstest.MapFS{}
	r := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := r.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		files[strings.TrimPrefix(hdr.Name, prefix)] = &fstest.MapFile{Data: data}
	}
}

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func PathInRepo(ctx context.Context, dir string) (string, error) {
	prefix, err := git(ctx, dir, "rev-parse", "--show-prefix")
	if err != nil {
		return "", fmt.Errorf("%s isn't inside a git repository", dir)
	}
	return strings.TrimSuffix(strings.TrimSpace(string(prefix)), "/"), nil
}
