package http

import (
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/gorilla/mux"
)

// mux.Router.Match runs each route's compiled regexp against the path, in registration order, and
// allocates the submatch indexes and the vars map for the route that wins. Most routes GoFr
// registers are a method and a template made of literal text and whole-segment {name} variables,
// which a string comparison per segment decides exactly as that regexp does. fastMatch scans the
// router's routes in mux's own order and decides those routes itself; it gives up and leaves the
// request to mux's Match the moment it meets a route it cannot decide the way mux would, and it
// never decides a miss. So mux still answers every 404 and 405, and every request a route of any
// other shape (a regexp variable, a host, a query, a prefix, a subrouter, a MatcherFunc) could
// win.
//
// A route is decided here only if all of these hold, checked once when the scan list is built:
//   - Add registered it, so its handler is an ownedHandler and its only matchers are one method
//     matcher and one path matcher;
//   - it has no build error and is not BuildOnly;
//   - its template is ASCII, and every variable is a whole segment with mux's default pattern;
//   - the regexp mux compiled for it is the one fastRoute's comparison is equivalent to, which
//     also rules out StrictSlash.
//
// Per request, the path must read the same encoded and decoded (so a route's UseEncodedPath does
// not matter), the router must hold as many routes as it did when the list was built (a route
// added later rebuilds it), and the router's middleware list must be what (*Router).Use leaves:
// dispatch or nothing.

// fastRoute is a route fastMatch decides: its method, and its template split around its
// variables. lits[0] precedes the first variable, lits[i] follows variable i-1.
type fastRoute struct {
	route   *mux.Route
	handler *ownedHandler
	method  string
	lits    []string
	names   []string
}

// fastRoutes is the scan list: one entry per route, in mux's order. A nil entry is a route
// fastMatch cannot decide.
type fastRoutes struct {
	entries []*fastRoute
	routes  int
}

// match reports whether path selects f. When vars is not nil it also stores the variable values
// in it as mux's extractVars does: in order, a later name overwriting an earlier one.
func (f *fastRoute) match(path string, vars map[string]string) bool {
	p, ok := strings.CutPrefix(path, f.lits[0])
	if !ok {
		return false
	}

	for i, lit := range f.lits[1:] {
		end := strings.IndexByte(p, '/')
		if end < 0 {
			end = len(p)
		}

		if end == 0 {
			return false
		}

		if vars != nil {
			vars[f.names[i]] = p[:end]
		}

		if p, ok = strings.CutPrefix(p[end:], lit); !ok {
			return false
		}
	}

	return p == ""
}

// value is the value path, which f matches, gives variable key in the map match fills: the last
// variable of that name, or "" when there is none.
func (f *fastRoute) value(path, key string) string {
	var v string

	p := path[len(f.lits[0]):]

	for i, lit := range f.lits[1:] {
		end := strings.IndexByte(p, '/')
		if end < 0 {
			end = len(p)
		}

		if f.names[i] == key {
			v = p[:end]
		}

		p = p[end+len(lit):]
	}

	return v
}

// muxFields reads the lengths of mux.Router's unexported route and middleware lists. Reading a
// length through reflect is allowed on an unexported field; nothing here reads or writes the
// elements.
type muxFields struct {
	routes, middlewares []int
}

//nolint:gochecknoglobals // field indexes into mux.Router, resolved once.
var muxRouterFields = func() *muxFields {
	t := reflect.TypeFor[mux.Router]()

	routes, ok1 := t.FieldByName("routes")
	mws, ok2 := t.FieldByName("middlewares")

	if !ok1 || !ok2 || routes.Type.Kind() != reflect.Slice || mws.Type.Kind() != reflect.Slice {
		return nil
	}

	return &muxFields{routes: routes.Index, middlewares: mws.Index}
}()

//nolint:gochecknoglobals // field indexes into mux.Route, resolved once.
var muxRouteFields = func() (matchers, buildOnly []int) {
	t := reflect.TypeFor[mux.Route]()

	m, ok1 := t.FieldByName("matchers")
	b, ok2 := t.FieldByName("buildOnly")

	if !ok1 || !ok2 || m.Type.Kind() != reflect.Slice || b.Type.Kind() != reflect.Bool {
		return nil, nil
	}

	return m.Index, b.Index
}

// fastState holds the scan list and the router state it was built against.
type fastState struct {
	list atomic.Pointer[fastRoutes]
	mu   sync.Mutex
}

// fastList returns the scan list for the router as it stands, or nil when fastMatch must not be
// used for this request.
func (rou *Router) fastList() *fastRoutes {
	if muxRouterFields == nil {
		return nil
	}

	v := reflect.ValueOf(&rou.Router).Elem()

	switch n := v.FieldByIndex(muxRouterFields.middlewares).Len(); {
	case n == 0 && !rou.dispatching, n == 1 && rou.dispatching:
	default:
		return nil
	}

	routes := v.FieldByIndex(muxRouterFields.routes).Len()

	if l := rou.fast.list.Load(); l != nil && l.routes == routes {
		return l
	}

	rou.fast.mu.Lock()
	defer rou.fast.mu.Unlock()

	if l := rou.fast.list.Load(); l != nil && l.routes == routes {
		return l
	}

	l := buildFastRoutes(&rou.Router, routes)
	rou.fast.list.Store(l)

	return l
}

func buildFastRoutes(r *mux.Router, routes int) *fastRoutes {
	matchersIdx, buildOnlyIdx := muxRouteFields()
	l := &fastRoutes{routes: routes, entries: make([]*fastRoute, 0, routes)}

	_ = r.Walk(func(route *mux.Route, _ *mux.Router, ancestors []*mux.Route) error {
		if len(ancestors) == 0 {
			l.entries = append(l.entries, newFastRoute(route, matchersIdx, buildOnlyIdx))
		}

		return nil
	})

	// Walk visits exactly the router's own routes at depth zero; anything else means the list
	// does not mirror mux's and must not be used.
	if len(l.entries) != routes {
		l.entries = []*fastRoute{nil}
	}

	return l
}

// newFastRoute returns the scan entry for route, or nil if fastMatch cannot decide it.
func newFastRoute(route *mux.Route, matchersIdx, buildOnlyIdx []int) *fastRoute {
	h, ok := route.GetHandler().(*ownedHandler)
	if !ok || !isMethodAndPath(route, matchersIdx, buildOnlyIdx) {
		return nil
	}

	methods, err := route.GetMethods()
	if err != nil || len(methods) != 1 {
		return nil
	}

	tpl, err := route.GetPathTemplate()
	if err != nil {
		return nil
	}

	re, err := route.GetPathRegexp()
	if err != nil {
		return nil
	}

	f := parseFastTemplate(tpl)
	if f == nil || f.regexp() != re {
		return nil
	}

	f.route, f.handler, f.method = route, h, methods[0]

	return f
}

// isMethodAndPath reports whether route can match, and has exactly two matchers: Add's method
// and path.
func isMethodAndPath(route *mux.Route, matchersIdx, buildOnlyIdx []int) bool {
	if matchersIdx == nil || route.GetError() != nil {
		return false
	}

	v := reflect.ValueOf(route).Elem()

	return !v.FieldByIndex(buildOnlyIdx).Bool() && v.FieldByIndex(matchersIdx).Len() == 2
}

// parseFastTemplate splits tpl into literals and whole-segment {name} variables, or returns nil.
func parseFastTemplate(tpl string) *fastRoute {
	if !isASCII(tpl) {
		return nil
	}

	f := &fastRoute{}
	rest := tpl

	for {
		lit, name, after, found := cutVar(rest)
		if !found {
			break
		}

		// A literal that follows a variable must start a new segment, or the variable's extent
		// would depend on more than the next slash.
		if name == "" || (len(f.names) > 0 && !strings.HasPrefix(lit, "/")) {
			return nil
		}

		f.lits = append(f.lits, lit)
		f.names = append(f.names, name)
		rest = after
	}

	if !isFastTail(rest, len(f.names) > 0) {
		return nil
	}

	f.lits = append(f.lits, rest)

	return f
}

// cutVar splits s around its first {name}. name is empty when the variable is not a plain name.
func cutVar(s string) (lit, name, after string, found bool) {
	open := strings.IndexByte(s, '{')
	if open < 0 {
		return "", "", "", false
	}

	end := strings.IndexByte(s[open:], '}')
	if end < 0 {
		return "", "", "", true
	}

	lit, name = s[:open], s[open+1:open+end]
	if strings.ContainsAny(name, "{:") || strings.IndexByte(lit, '}') >= 0 {
		name = ""
	}

	return lit, name, s[open+end+1:], true
}

// isFastTail reports whether rest can end a template: no stray brace, and nothing glued to a
// variable before it.
func isFastTail(rest string, afterVar bool) bool {
	return !strings.ContainsAny(rest, "{}") && (!afterVar || rest == "" || rest[0] == '/')
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

// regexp is the expression mux compiles for f's template with its default settings.
func (f *fastRoute) regexp() string {
	var b strings.Builder

	b.WriteByte('^')

	for i := range f.names {
		b.WriteString(regexp.QuoteMeta(f.lits[i]))
		b.WriteString("(?P<v")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(">[^/]+)")
	}

	b.WriteString(regexp.QuoteMeta(f.lits[len(f.lits)-1]))
	b.WriteByte('$')

	return b.String()
}

// fastMatch decides r against the scan list. It reports true with the route, the handler mux
// would have returned and the path its variables are read from in s, and false whenever mux must decide.
func (rou *Router) fastMatch(r *http.Request, s *muxState) bool {
	l := rou.fastList()
	if l == nil || r.URL.EscapedPath() != r.URL.Path {
		return false
	}

	path, method := r.URL.Path, r.Method

	for _, f := range l.entries {
		if f == nil {
			return false
		}

		if f.method != method || !f.match(path, nil) {
			continue
		}

		s.match.Route = f.route
		s.match.Handler = f.handler
		s.fast = f
		s.path = path

		if rou.dispatching {
			s.match.Handler = rou.dispatch(f.handler)
		}

		return true
	}

	return false
}
