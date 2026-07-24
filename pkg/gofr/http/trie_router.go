package http

import (
	"errors"
	"net/http"
	"regexp/syntax"
	"sort"
	"strings"

	"github.com/gorilla/mux"
)

// routeEntry pairs a registered mux route with its registration order. The
// order lets the index reproduce mux's "first registered wins" semantics after
// the trie has narrowed the candidate set.
type routeEntry struct {
	route *mux.Route
	order int
}

// trieNode indexes routes by path segment. A literal segment ("users") is keyed
// in children; a parameter segment ("{id}" or "{id:[0-9]+}") uses paramChild.
// Routes whose template ends at this node are collected in routes.
type trieNode struct {
	children   map[string]*trieNode
	paramChild *trieNode
	routes     []*routeEntry
}

func newTrieNode() *trieNode { return &trieNode{children: make(map[string]*trieNode)} }

// routeIndex accelerates route matching to O(path length) — independent of the
// number of registered routes — by narrowing the candidate set with a segment
// trie and then delegating the ACTUAL match to mux's own Route.Match. Because
// the final decision is still mux's, every mux semantic (method matching,
// {id:regex} constraints, header/query/host matchers, path cleaning) is
// preserved exactly; the trie only avoids mux's O(n) linear scan.
//
// The O(path length) property applies to trie-indexable routes (plain exact
// paths). Routes that cannot be indexed (PathPrefix/static, slash-spanning
// params) live in the fallback list, which is scanned on every request — so an
// app that registers many such routes stays linear in the size of that set. In
// practice the fallback set is small (a handful of static/catch-all routes), so
// the "flat as route count grows" result holds for realistic route tables.
type routeIndex struct {
	root     *trieNode
	fallback []*routeEntry // routes with no indexable path template (PathPrefix, host-only, ...)
}

func newRouteIndex() *routeIndex { return &routeIndex{root: newTrieNode()} }

// build indexes every route the given mux router knows about, in registration
// order. Routes with a plain segmented path go into the trie; anything else
// (PathPrefix, host-only, matcher-only) goes into the order-preserving fallback
// list so it is still considered on every request.
func (idx *routeIndex) build(router *mux.Router) {
	order := 0

	// Walk visits routes in registration order. The walk func never returns an
	// error, so the walk always completes and every route is classified.
	_ = router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		e := &routeEntry{route: route, order: order}
		order++

		tpl, ok := exactPathTemplate(route)
		if !ok {
			idx.fallback = append(idx.fallback, e)

			return nil
		}

		idx.insert(tpl, e)

		return nil
	})
}

// exactPathTemplate returns the path template of route and true only when the
// route can be safely trie-indexed: a full, exact path whose every segment maps
// to exactly one request-path segment. It returns false — deferring the route
// to the always-evaluated fallback list, where mux's own Route.Match decides it
// — for:
//
//   - PathPrefix/static handlers (un-anchored path regexp);
//   - routes whose path template does not start with "/" or has no path at all;
//   - routes with a parameter whose regex can match "/" (e.g. "{path:.*}"), which
//     can span multiple request-path segments and therefore cannot be located by
//     the trie's segment-by-segment walk;
//   - routes with a mixed literal+parameter segment (e.g. "/{name}.txt" or
//     "/user-{id}"), which the whole-segment trie keys cannot represent.
//
// Trie membership is purely an acceleration decision: correctness never depends
// on it, because match() runs mux's real Route.Match on whichever candidate is
// selected regardless of which list it came from. Host/header/query matchers on
// an otherwise-exact path are thus safe to index — Route.Match still enforces
// them — so they are intentionally not excluded here.
//
// The exact-vs-prefix discriminator is the path regexp: mux end-anchors an exact
// path ("^/users/([^/]+)$") but leaves a PathPrefix un-anchored ("^/static/").
func exactPathTemplate(route *mux.Route) (string, bool) {
	tpl, err := route.GetPathTemplate()
	if err != nil || !strings.HasPrefix(tpl, "/") {
		return "", false
	}

	rx, err := route.GetPathRegexp()
	if err != nil || !strings.HasSuffix(rx, "$") {
		return "", false
	}

	// A parameter whose regex can match "/" (e.g. "{path:.*}") spans multiple
	// request-path segments and cannot be located by the segment-by-segment
	// trie walk, so such a route must go to the fallback list. The literal "/"
	// separators live at the top level of the path regexp; only the capture
	// groups (the params) are inspected here.
	if pathRegexpHasSlashInCapture(rx) {
		return "", false
	}

	// The trie keys each segment as a whole — either a literal ("users") or a
	// single parameter ("{id}"). A segment that mixes a literal with a parameter
	// ("{name}.txt", "user-{id}", "v{ver}") is neither, so it cannot be indexed
	// by a whole-segment key; defer such routes to the fallback list.
	for _, seg := range pathSegments(tpl) {
		if strings.Contains(seg, "{") && !isParamSegment(seg) {
			return "", false
		}
	}

	return tpl, true
}

// pathRegexpHasSlashInCapture reports whether any capturing group in the mux
// path regexp rx can match a "/". It over-approximates: a regexp it cannot parse
// is treated as slash-capable, so a route is trie-indexed only when its params
// are provably slash-free (no false negatives — a spanning route is never wrongly
// indexed and silently lost).
func pathRegexpHasSlashInCapture(rx string) bool {
	re, err := syntax.Parse(rx, syntax.Perl)
	if err != nil {
		return true
	}

	return anyCaptureMatchesSlash(re)
}

// anyCaptureMatchesSlash walks re and reports whether any OpCapture subtree can
// match a "/".
func anyCaptureMatchesSlash(re *syntax.Regexp) bool {
	if re.Op == syntax.OpCapture && reNodeMayMatchSlash(re) {
		return true
	}

	for _, sub := range re.Sub {
		if anyCaptureMatchesSlash(sub) {
			return true
		}
	}

	return false
}

// reNodeMayMatchSlash reports whether the regexp subtree re can match a "/".
func reNodeMayMatchSlash(re *syntax.Regexp) bool {
	if opEmitsSlash(re) {
		return true
	}

	for _, sub := range re.Sub {
		if reNodeMayMatchSlash(sub) {
			return true
		}
	}

	return false
}

// opEmitsSlash reports whether this single regexp node (ignoring its children)
// can itself contribute a "/". Only literal/char-class/any-char ops can; every
// other op is purely structural and defers to its children.
//
//nolint:exhaustive // only the char-producing ops can introduce a "/"; the rest are structural.
func opEmitsSlash(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		// "." matches "/" in Go's regexp (only newline is excluded).
		return true
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			if r == '/' {
				return true
			}
		}
	case syntax.OpCharClass:
		// Rune holds inclusive [lo, hi] range pairs.
		for i := 0; i+1 < len(re.Rune); i += 2 {
			if re.Rune[i] <= '/' && '/' <= re.Rune[i+1] {
				return true
			}
		}
	}

	return false
}

func (idx *routeIndex) insert(tpl string, e *routeEntry) {
	cur := idx.root

	for _, seg := range pathSegments(tpl) {
		if isParamSegment(seg) {
			if cur.paramChild == nil {
				cur.paramChild = newTrieNode()
			}

			cur = cur.paramChild

			continue
		}

		next, ok := cur.children[seg]
		if !ok {
			next = newTrieNode()
			cur.children[seg] = next
		}

		cur = next
	}

	cur.routes = append(cur.routes, e)
}

// candidates gathers every route whose template structurally lines up with path
// by exploring both the literal and the parameter branch at each segment
// (backtracking). This is O(path length × small branching), never O(route
// count). A route the trie omits here could not have matched path anyway, so
// omitting it does not change the final result.
func (idx *routeIndex) candidates(path string) []*routeEntry {
	var out []*routeEntry

	var walk func(n *trieNode, segs []string)

	walk = func(n *trieNode, segs []string) {
		if len(segs) == 0 {
			out = append(out, n.routes...)

			return
		}

		seg, rest := segs[0], segs[1:]

		if child, ok := n.children[seg]; ok {
			walk(child, rest)
		}

		if n.paramChild != nil {
			walk(n.paramChild, rest)
		}
	}

	walk(idx.root, pathSegments(path))

	return out
}

// match narrows candidates via the trie, adds the fallback routes, orders the
// whole set by registration order, and returns mux's own match for the first
// candidate that fully matches — identical to what stock mux would pick, only
// without scanning every registered route.
//
// It reports true and fills rm on a full match. On no match it returns false;
// if some candidate matched the path but not the method, rm.MatchErr is set to
// mux.ErrMethodMismatch so the caller can render 405 rather than 404.
//
// Note: inside a running GoFr app this 405 path is effectively unreachable. GoFr
// registers a PathPrefix("/") catch-all (see gofr.go) that matches any method,
// so a wrong-method request to an existing path is a FULL match on the catch-all
// and returns before methodMismatch is consulted — yielding the same 404 as mux.
// The 405 result only surfaces on a bare Router with no catch-all (e.g. unit
// tests), which is why the documented 404->405 divergence is framework-moot.
func (idx *routeIndex) match(req *http.Request, rm *mux.RouteMatch) bool {
	cands := idx.candidates(req.URL.Path)
	cands = append(cands, idx.fallback...)

	if len(cands) > 1 {
		sort.Slice(cands, func(i, j int) bool { return cands[i].order < cands[j].order })
	}

	methodMismatch := false

	for _, e := range cands {
		var m mux.RouteMatch

		if e.route.Match(req, &m) && m.MatchErr == nil {
			*rm = m

			return true
		}

		if errors.Is(m.MatchErr, mux.ErrMethodMismatch) {
			methodMismatch = true
		}
	}

	if methodMismatch {
		rm.MatchErr = mux.ErrMethodMismatch
	}

	return false
}

// pathSegments splits a path into its non-empty segments. "/" yields no
// segments; "/users/{id}" yields ["users", "{id}"].
func pathSegments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}

	return strings.Split(p, "/")
}

// isParamSegment reports whether a template segment is a mux parameter, i.e.
// "{name}" or "{name:regex}".
func isParamSegment(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}
