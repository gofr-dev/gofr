package mcp

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"sync"
)

// Learner watches handler invocations and records the Go types that
// each route binds, returns, and reads as query parameters. The
// recorded types feed the MCP tool manifest, so the LLM receives
// JSON Schemas that match the developer's actual structs without
// any annotation work.
//
// Schemas can be persisted to a file so a fresh process boot has full
// schemas immediately, before any traffic.
type Learner struct {
	mu        sync.RWMutex
	schemas   map[string]*RouteSchema
	persistTo string
}

// RouteSchema is what we know about a single registered route. The
// fields are exported so the persisted JSON is greppable by humans
// debugging tool definitions.
type RouteSchema struct {
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	PathParams []string `json:"pathParams,omitempty"`
	QueryKeys  []string `json:"queryKeys,omitempty"`
	BodyType   string   `json:"bodyType,omitempty"` // diagnostic
	Body       Schema   `json:"body,omitempty"`
	Output     Schema   `json:"output,omitempty"`
}

// NewLearner returns a Learner with no schemas. If persistTo is
// non-empty, schemas are loaded from that file at construction and
// can be flushed back via Save.
func NewLearner(persistTo string) *Learner {
	l := &Learner{
		schemas:   map[string]*RouteSchema{},
		persistTo: persistTo,
	}
	if persistTo != "" {
		l.load()
	}

	return l
}

// Register seeds the manifest with a route discovered at startup. The
// schema details (body, output) are filled in lazily on first call.
// Calling Register on an already-known route is a no-op so persisted
// schemas survive a restart.
func (l *Learner) Register(method, path string, pathParams []string) {
	key := routeKey(method, path)

	l.mu.Lock()
	defer l.mu.Unlock()

	if existing, ok := l.schemas[key]; ok {
		// Path params come from the mux template — always trust the
		// fresh value over what was persisted.
		existing.PathParams = pathParams

		return
	}

	l.schemas[key] = &RouteSchema{
		Method:     method,
		Path:       path,
		PathParams: pathParams,
	}
}

// RecordBind is called by the request wrapper after a successful
// c.Bind(&u) — t is the underlying type behind &u.
func (l *Learner) RecordBind(method, path string, t reflect.Type) {
	if t == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	rs, ok := l.schemas[routeKey(method, path)]
	if !ok {
		return
	}

	rs.BodyType = t.String()
	rs.Body = FromType(t)
}

// RecordQueryKey records that a handler read this query key. Insertion
// order is preserved so the resulting schema is deterministic.
func (l *Learner) RecordQueryKey(method, path, key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	rs, ok := l.schemas[routeKey(method, path)]
	if !ok {
		return
	}

	if slices.Contains(rs.QueryKeys, key) {
		return
	}

	rs.QueryKeys = append(rs.QueryKeys, key)
}

// RecordReturn captures the type the handler returned. Called from the
// request wrapper after the handler completes. nil returns are ignored
// so we don't overwrite a previously-learned shape.
func (l *Learner) RecordReturn(method, path string, v any) {
	if v == nil {
		return
	}

	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	rs, ok := l.schemas[routeKey(method, path)]
	if !ok {
		return
	}

	rs.Output = FromType(t)
}

// Schemas returns a snapshot of all known route schemas, keyed by
// "METHOD path".
func (l *Learner) Schemas() map[string]RouteSchema {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make(map[string]RouteSchema, len(l.schemas))
	for k, v := range l.schemas {
		out[k] = *v
	}

	return out
}

// Save writes learned schemas to persistTo, if configured. Safe to
// call at shutdown so the next process boot has warm schemas.
func (l *Learner) Save() error {
	if l.persistTo == "" {
		return nil
	}

	l.mu.RLock()
	data, err := json.MarshalIndent(l.schemas, "", "  ")
	l.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(l.persistTo, data, 0o600)
}

func (l *Learner) load() {
	data, err := os.ReadFile(l.persistTo)
	if err != nil {
		return
	}

	_ = json.Unmarshal(data, &l.schemas)
}

func routeKey(method, path string) string {
	return method + " " + path
}
