package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

var (
	ErrUnknownRoute = errors.New("unknown route")
	ErrUnknownState = errors.New("unknown state")
)

type lookupError struct {
	msg  string
	kind error
}

func (e *lookupError) Error() string { return e.msg }

func (e *lookupError) Unwrap() error { return e.kind }

func SetActive(dir, routeID, state string) (string, error) {
	proj, probs, err := Load(dir)
	if err != nil {
		return "", fmt.Errorf("can't read project folder %s: %w", dir, err)
	}
	r, err := FindRoute(proj, routeID)
	if err != nil {
		return "", err
	}
	if own := problemsIn(probs, r.Src.File); len(own) > 0 {
		return "", fmt.Errorf("%s has problems; fix them first:\n  %s", r.Src.File, strings.ReplaceAll(own.String(), "\n", "\n  "))
	}
	if _, ok := r.States.Get(state); !ok {
		names := r.States.Names()
		return "", &lookupError{
			msg:  fmt.Sprintf("route %s has no state %q (%s)", routeID, state, closeMatchOr(state, names, "states: "+strings.Join(names, ", "))),
			kind: ErrUnknownState,
		}
	}
	if r.Active == state {
		return r.Active, nil
	}

	path := filepath.Join(dir, filepath.FromSlash(r.Src.File))
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	updated, err := setActiveIn(data, strings.TrimPrefix(r.ID, r.Group+"."), state)
	if err != nil {
		return "", fmt.Errorf("route %s %w", routeID, err)
	}
	return r.Active, WriteFileAtomic(path, updated, info.Mode().Perm())
}

func routeIDs(p *model.Project) []string {
	ids := make([]string, len(p.Routes))
	for i, r := range p.Routes {
		ids[i] = r.ID
	}
	return ids
}

func problemsIn(probs Problems, file string) Problems {
	var own Problems
	for _, p := range probs {
		if p.File == file {
			own = append(own, p)
		}
	}
	return own
}

var errOneLineRoute = errors.New("is written on one line")

func setActiveIn(data []byte, routeName, state string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var route *yaml.Node
	for key, val := range pairs(doc.Content[0]) {
		if key.Value == routeName {
			route = val
		}
	}
	var active, statesKey *yaml.Node
	for key, val := range pairs(route) {
		switch key.Value {
		case "active":
			active = val
		case "states":
			statesKey = key
		}
	}
	if active != nil {
		start := lineOffset(data, active.Line) + active.Column - 1
		end := start + tokenLength(data[start:], active.Style)
		return bytes.Join([][]byte{data[:start], []byte(state), data[end:]}, nil), nil
	}
	if route.Style&yaml.FlowStyle != 0 {
		return nil, fmt.Errorf("%w; add \"active: %s\" to it by hand", errOneLineRoute, state)
	}
	at := lineOffset(data, statesKey.Line)
	indent := data[at : at+statesKey.Column-1]
	eol := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r\n"
	}
	line := string(indent) + "active: " + state + eol
	return bytes.Join([][]byte{data[:at], []byte(line), data[at:]}, nil), nil
}

func lineOffset(data []byte, line int) int {
	offset := 0
	for range line - 1 {
		offset += bytes.IndexByte(data[offset:], '\n') + 1
	}
	return offset
}

func tokenLength(rest []byte, style yaml.Style) int {
	if style == yaml.DoubleQuotedStyle || style == yaml.SingleQuotedStyle {
		return bytes.IndexByte(rest[1:], rest[0]) + 2
	}
	line := rest
	if i := bytes.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	if i := bytes.Index(line, []byte(" #")); i >= 0 {
		line = line[:i]
	}
	if i := bytes.IndexAny(line, ",}"); i >= 0 {
		line = line[:i]
	}
	return len(bytes.TrimRight(line, " \t"))
}

func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	_, writeErr := tmp.Write(data)
	if err := errors.Join(writeErr, tmp.Chmod(perm), tmp.Sync(), tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func FindRoute(p *model.Project, id string) (*model.Route, error) {
	if r, ok := p.Route(id); ok {
		return r, nil
	}
	ids := routeIDs(p)
	return nil, &lookupError{
		msg:  fmt.Sprintf("no route %q (%s)", id, closeMatchOr(id, ids, "routes: "+strings.Join(ids, ", "))),
		kind: ErrUnknownRoute,
	}
}
