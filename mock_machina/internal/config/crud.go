package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

var (
	crudFields     = []string{"collection", "idField"}
	collectionName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

func (l *loader) crud(f map[string]*yaml.Node, r *model.Route) {
	v := f["crud"]
	if r.Method != model.MethodCRUD {
		if v != nil {
			l.add(r.Src.File, v.Line, "crud only applies to CRUD routes, like route: CRUD /things")
		}
		return
	}
	segments := strings.Split(strings.Trim(r.Path, "/"), "/")
	c := &model.CRUD{Collection: segments[len(segments)-1], IDField: "id"}
	if v != nil && !l.crudOptions(r.Src.File, v, c) {
		return
	}
	c.Data = l.collectionData(c)
	r.CRUD = c
}

func (l *loader) crudOptions(file string, v *yaml.Node, c *model.CRUD) bool {
	if v.Kind != yaml.MappingNode {
		l.add(file, v.Line, "crud must be a mapping, like { collection: cart_items, idField: id }")
		return false
	}
	fields := l.fields(file, v, crudFields, nil)
	if n := fields["collection"]; n != nil {
		c.Collection = n.Value
	}
	if n := fields["idField"]; n != nil {
		c.IDField = n.Value
	}
	if !collectionName.MatchString(c.Collection) {
		l.add(file, v.Line, "crud collection %q must use lowercase letters, digits, dashes and underscores", c.Collection)
		return false
	}
	return true
}

func (l *loader) collectionData(c *model.CRUD) []byte {
	file := path.Join("data", c.Collection+".json")
	if data, seen := l.data[file]; seen {
		return data
	}
	if l.data == nil {
		l.data = map[string][]byte{}
	}
	data := l.readCollection(file, c.IDField)
	l.data[file] = data
	return data
}

func (l *loader) readCollection(file, idField string) []byte {
	data, err := fs.ReadFile(l.fsys, file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		l.add(file, 0, "can't read this file: %v", err)
		return nil
	}
	var items []map[string]json.RawMessage
	if json.NewDecoder(bytes.NewReader(data)).Decode(&items) != nil {
		l.add(file, 0, "must be a JSON list of objects, like [{%q: \"t_1\"}]", idField)
		return nil
	}
	for i, item := range items {
		if _, ok := item[idField]; !ok {
			l.add(file, 0, "item %d has no %q", i+1, idField)
			return nil
		}
	}
	return data
}
