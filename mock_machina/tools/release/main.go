package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/demola234/tiny-tools/mock_machina/internal/release"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:])
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: release build|notes --version vX.Y.Z [--out dist] [--repo owner/name]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	version := fs.String("version", "", "version to release, like v0.7.0")
	out := fs.String("out", "dist", "folder for the archives and files")
	repo := fs.String("repo", "demola234/tiny-tools", "GitHub repository the release lives in")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *version == "" {
		return errors.New("--version is required")
	}
	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		return err
	}
	notes, err := release.Notes(changelog, *version)
	if err != nil {
		return err
	}
	switch args[0] {
	case "notes":
		_, err = fmt.Fprint(os.Stdout, notes)
		return err
	case "build":
		return build(ctx, *version, *out, *repo, notes)
	}
	return fmt.Errorf("unknown command %q; use build or notes", args[0])
}

func build(ctx context.Context, version, out, repo, notes string) error {
	arts, err := release.Build(ctx, release.Options{Version: version, ModuleDir: ".", Out: out, Targets: release.Targets})
	if err != nil {
		return err
	}
	files := map[string][]byte{
		"mockmachina.rb":   release.Formula(version, repo, arts),
		"mockmachina.json": release.Scoop(version, repo, arts),
		"notes.md":         []byte(notes),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil { //nolint:gosec // out is the maintainer's chosen output folder
			return err
		}
	}
	for _, a := range arts {
		fmt.Fprintln(os.Stdout, filepath.Join(out, a.Name))
	}
	return nil
}
