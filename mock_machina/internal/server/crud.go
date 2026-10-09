package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"sync"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
)

const crudParam = "crud_id"

type item struct {
	keys   []string
	values map[string]json.RawMessage
}

type collection struct {
	mu    sync.Mutex
	data  []byte
	items []*item
}

func newCollection(data []byte, idField string) *collection {
	c := &collection{data: data}
	var raw []json.RawMessage
	_ = json.Unmarshal(data, &raw)
	for _, r := range raw {
		if it, err := decodeItem(r); err == nil && it.values[idField] != nil {
			c.items = append(c.items, it)
		}
	}
	return c
}

func (m *Memory) collection(name string, data []byte, idField string) *collection {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.collections[name]; ok && bytes.Equal(c.data, data) {
		return c
	}
	c := newCollection(data, idField)
	m.collections[name] = c
	return c
}

var errNotObject = errors.New("the body must be a JSON object")

func decodeItem(data []byte) (*item, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errNotObject
	}
	it := &item{values: map[string]json.RawMessage{}}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, errNotObject
		}
		key, _ := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, errNotObject
		}
		it.set(key, v)
	}
	return it, nil
}

func (it *item) set(key string, v json.RawMessage) {
	if _, ok := it.values[key]; !ok {
		it.keys = append(it.keys, key)
	}
	it.values[key] = v
}

func (it *item) encode(b *bytes.Buffer) {
	b.WriteByte('{')
	for i, k := range it.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := json.Marshal(k)
		b.Write(key)
		b.WriteByte(':')
		b.Write(it.values[k])
	}
	b.WriteByte('}')
}

func rawText(v json.RawMessage) string {
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	return string(v)
}

func (c *collection) find(idField, id string) int {
	return slices.IndexFunc(c.items, func(it *item) bool { return rawText(it.values[idField]) == id })
}

type crudResult struct {
	status   int
	body     []byte
	location string
}

func (x *exchange) crud(st *model.State) bool {
	return x.rt.CRUD != nil && st.Status == 0 && st.Body.Data == nil && st.Fault == nil && len(st.Headers) == 0
}

func (x *exchange) serveCRUD() crudResult {
	cfg := x.rt.CRUD
	c := x.opts.Memory.collection(cfg.Collection, cfg.Data, cfg.IDField)
	c.mu.Lock()
	defer c.mu.Unlock()
	id := x.req.PathValue(crudParam)
	if id == "" {
		if x.req.Method == http.MethodPost {
			return x.create(c)
		}
		return list(c, x.req)
	}
	i := c.find(cfg.IDField, id)
	if i < 0 {
		return problem(http.StatusNotFound, map[string]string{"error": "not_found", "id": id})
	}
	switch x.req.Method {
	case http.MethodDelete:
		c.items = slices.Delete(c.items, i, i+1)
		return crudResult{status: http.StatusNoContent}
	case http.MethodPut, http.MethodPatch:
		return x.update(c, i)
	default:
		return itemResult(http.StatusOK, c.items[i])
	}
}

func list(c *collection, req *http.Request) crudResult {
	var b bytes.Buffer
	b.WriteByte('[')
	n := 0
	for _, it := range c.items {
		if !matchesQuery(it, req) {
			continue
		}
		if n > 0 {
			b.WriteByte(',')
		}
		it.encode(&b)
		n++
	}
	b.WriteByte(']')
	return crudResult{status: http.StatusOK, body: b.Bytes()}
}

func matchesQuery(it *item, req *http.Request) bool {
	for key, values := range req.URL.Query() {
		if key == stateQuery {
			continue
		}
		v, ok := it.values[key]
		if !ok || rawText(v) != values[0] {
			return false
		}
	}
	return true
}

func (x *exchange) readItem() (*item, *crudResult) {
	it, err := decodeItem(x.body())
	if err != nil {
		r := problem(http.StatusBadRequest, map[string]string{"error": "invalid_body", "message": err.Error()})
		return nil, &r
	}
	return it, nil
}

func (x *exchange) create(c *collection) crudResult {
	it, bad := x.readItem()
	if bad != nil {
		return *bad
	}
	idField := x.rt.CRUD.IDField
	if v, ok := it.values[idField]; ok {
		if c.find(idField, rawText(v)) >= 0 {
			return problem(http.StatusConflict, map[string]string{"error": "conflict", "id": rawText(v)})
		}
	} else {
		id, _ := json.Marshal(seed.UUID(x.opts.Seed.Stream(x.rt.ID, "crud", strconv.Itoa(x.call))))
		it.keys = append([]string{idField}, it.keys...)
		it.values[idField] = id
	}
	c.items = append(c.items, it)
	r := itemResult(http.StatusCreated, it)
	r.location = x.rt.Path + "/" + rawText(it.values[idField])
	return r
}

func (x *exchange) update(c *collection, i int) crudResult {
	body, bad := x.readItem()
	if bad != nil {
		return *bad
	}
	idField := x.rt.CRUD.IDField
	current := c.items[i]
	next := body
	if x.req.Method == http.MethodPatch {
		next = &item{keys: slices.Clone(current.keys), values: maps.Clone(current.values)}
		for _, k := range body.keys {
			if k != idField {
				next.set(k, body.values[k])
			}
		}
	}
	if _, ok := next.values[idField]; !ok {
		next.keys = append([]string{idField}, next.keys...)
	}
	next.values[idField] = current.values[idField]
	c.items[i] = next
	return itemResult(http.StatusOK, next)
}

func itemResult(status int, it *item) crudResult {
	var b bytes.Buffer
	it.encode(&b)
	return crudResult{status: status, body: b.Bytes()}
}

func problem(status int, v map[string]string) crudResult {
	var b bytes.Buffer
	_ = json.NewEncoder(&b).Encode(v)
	return crudResult{status: status, body: b.Bytes()}
}
