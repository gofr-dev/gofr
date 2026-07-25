package http

import (
	"errors"
	"net/http"
	"regexp/syntax"
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

// exactPathTemplate returns the path template of route, and true only when the
// route's shape can be proven trie-indexable by isIndexablePathRegexp — a fully
// anchored path whose every segment maps to exactly one request-path segment.
// Everything else goes to the always-evaluated fallback list, where mux's own
// Route.Match decides it.
//
// A segment that mixes a literal with a parameter ("{name}.txt", "user-{id}") is
// still indexable: it matches exactly one path segment, so it is inserted as a
// parameter slot (see segmentHasParam) and mux validates the intra-segment
// pattern.
//
// Trie membership is purely an acceleration decision: correctness never depends
// on it, because match() runs mux's real Route.Match on whichever candidate is
// selected regardless of which list it came from. Host/header/query matchers on
// an otherwise-exact path are thus safe to index — Route.Match still enforces
// them — so they are intentionally not excluded here.
func exactPathTemplate(route *mux.Route) (string, bool) {
	tpl, err := route.GetPathTemplate()
	if err != nil || !strings.HasPrefix(tpl, "/") {
		return "", false
	}

	rx, err := route.GetPathRegexp()
	if err != nil {
		return "", false
	}

	if !isIndexablePathRegexp(rx) {
		return "", false
	}

	return tpl, true
}

// isIndexablePathRegexp decides, from mux's compiled path regexp, whether a
// route's shape is one the segment trie can locate. It is an ALLOWLIST: it
// returns true only for a pattern it can positively prove is
//
//   - anchored at both ends (an exact path, not a PathPrefix), and
//   - built from captures that each match a non-empty, slash-free fragment,
//
// so the template's segment count always equals the request's segment count.
// Anything it cannot prove — a regexp it cannot parse, a prefix pattern, a capture
// that may span "/" ("{p:.*}") or match "" ("{p:[a-z]*}") — returns false and
// the route goes to the fallback list, where mux's own Route.Match decides it.
//
// An allowlist is deliberate. Trie membership is only an acceleration decision
// (match() always runs mux's real Route.Match on the candidates it produces), so
// a route wrongly EXCLUDED merely loses speed, while a route wrongly INCLUDED is
// a correctness bug — it can be dropped from the candidate set entirely and
// silently 404 or fall through to another handler. Proving the shape is safe,
// rather than enumerating unsafe shapes, keeps that failure mode impossible.
//
// Anchoring is checked on the parsed syntax tree, not by a "$" string suffix: a
// template containing a literal "$" is QuoteMeta-escaped by mux, so a PathPrefix
// such as "/p$" compiles to the unanchored "^/p\\$" — which ends in the byte "$"
// yet must NOT be treated as an exact path.
func isIndexablePathRegexp(rx string) bool {
	re, err := syntax.Parse(rx, syntax.Perl)
	if err != nil {
		return false
	}

	return isAnchoredBothEnds(re) && capturesAreSingleSegment(re)
}

// isAnchoredBothEnds reports whether re begins with a begin-text anchor and ends
// with an end-text anchor — mux's shape for an exact path ("^/users/([^/]+)$")
// as opposed to a prefix ("^/static/").
func isAnchoredBothEnds(re *syntax.Regexp) bool {
	if re.Op != syntax.OpConcat || len(re.Sub) < 2 {
		return false
	}

	first, last := re.Sub[0], re.Sub[len(re.Sub)-1]

	beginOK := first.Op == syntax.OpBeginText || first.Op == syntax.OpBeginLine
	endOK := last.Op == syntax.OpEndText || last.Op == syntax.OpEndLine

	return beginOK && endOK
}

// capturesAreSingleSegment reports whether every capturing group in re matches a
// non-empty, slash-free fragment — i.e. each parameter consumes text within
// exactly one path segment, never spanning "/" and never vanishing.
func capturesAreSingleSegment(re *syntax.Regexp) bool {
	if re.Op == syntax.OpCapture && (reNodeMayMatchSlash(re) || canMatchEmpty(re)) {
		return false
	}

	for _, sub := range re.Sub {
		if !capturesAreSingleSegment(sub) {
			return false
		}
	}

	return true
}

// canMatchEmpty reports whether the regexp subtree re can match the empty
// string. Such a capture would let a template segment vanish, so the template's
// segment count would no longer equal the request's and the trie walk would miss
// the route (e.g. "/{s:[a-z]*}" must match the request "/"). Unrecognized ops
// are reported as empty-matching, keeping the decision conservative.
//
//nolint:exhaustive // the default arm deliberately covers every remaining op conservatively.
func canMatchEmpty(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpLiteral:
		return len(re.Rune) == 0
	case syntax.OpCharClass, syntax.OpAnyChar, syntax.OpAnyCharNotNL, syntax.OpNoMatch:
		// Each of these consumes exactly one rune.
		return false
	case syntax.OpCapture, syntax.OpPlus:
		// x+ matches empty only if x does; a capture defers to its body.
		return canMatchEmpty(re.Sub[0])
	case syntax.OpRepeat:
		return re.Min == 0 || canMatchEmpty(re.Sub[0])
	case syntax.OpConcat:
		return allCanMatchEmpty(re.Sub)
	case syntax.OpAlternate:
		return anyCanMatchEmpty(re.Sub)
	default:
		// OpEmptyMatch, OpStar, OpQuest, anchors, word boundaries, and anything
		// added to the syntax package later: assume it can match empty.
		return true
	}
}

// allCanMatchEmpty reports whether every branch can match empty — a concatenation
// matches empty only if all of its parts do.
func allCanMatchEmpty(subs []*syntax.Regexp) bool {
	for _, sub := range subs {
		if !canMatchEmpty(sub) {
			return false
		}
	}

	return true
}

// anyCanMatchEmpty reports whether any branch can match empty — an alternation
// matches empty if at least one of its arms does.
func anyCanMatchEmpty(subs []*syntax.Regexp) bool {
	for _, sub := range subs {
		if canMatchEmpty(sub) {
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
		if segmentHasParam(seg) {
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
func (n *trieNode) collect(rest string, out *[]*routeEntry) {
	if rest == "" {
		*out = append(*out, n.routes...)

		return
	}

	var seg, tail string
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		seg, tail = rest[:i], rest[i+1:]
	} else {
		seg, tail = rest, ""
	}

	if child, ok := n.children[seg]; ok {
		child.collect(tail, out)
	}

	if n.paramChild != nil {
		n.paramChild.collect(tail, out)
	}
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
	var buf [12]*routeEntry

	cands := buf[:0]
	idx.root.collect(strings.Trim(req.URL.Path, "/"), &cands)

	// mux matches the escaped path when the router is in UseEncodedPath mode,
	// where the escaped and decoded forms can split into different segments.
	// That flag is not readable from here, so when the two forms differ, walk
	// both and take the union. Over-producing candidates is always safe (mux
	// filters them); missing one would not be.
	if esc := req.URL.EscapedPath(); esc != req.URL.Path {
		idx.root.collect(strings.Trim(esc, "/"), &cands)
	}

	if len(idx.fallback) > 0 {
		cands = append(cands, idx.fallback...)
	}

	sortByRegistrationOrder(cands)

	methodMismatch := false

	for _, e := range cands {
		// Match writes into rm directly; reset it between candidates so a
		// failed attempt cannot leak state into the next one. This avoids a
		// per-candidate RouteMatch allocation.
		*rm = mux.RouteMatch{}

		// Mirror mux.Router.ServeHTTP: the first route whose Match succeeds
		// wins, whatever MatchErr says. A subrouter with its own
		// NotFoundHandler reports a successful match with MatchErr set to
		// ErrNotFound and its handler in rm.Handler, and mux serves exactly
		// that; requiring MatchErr == nil here would skip it and fall through
		// to an unrelated route.
		if e.route.Match(req, rm) {
			return true
		}

		if errors.Is(rm.MatchErr, mux.ErrMethodMismatch) {
			methodMismatch = true
		}
	}

	*rm = mux.RouteMatch{}

	if methodMismatch {
		rm.MatchErr = mux.ErrMethodMismatch
	}

	return false
}

// sortByRegistrationOrder orders candidates by their registration index so the
// scan reproduces mux's first-registered-wins semantics. Insertion sort: the
// candidate set is tiny (a handful at most) and, unlike sort.Slice, this
// allocates nothing — sort.Slice's reflect-based swapper would otherwise cost an
// allocation on every single request, since GoFr's PathPrefix("/") catch-all
// means the set is essentially never of length one.
func sortByRegistrationOrder(cands []*routeEntry) {
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && cands[j].order < cands[j-1].order; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
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

// segmentHasParam reports whether a template segment contains a mux parameter —
// a whole-segment param ("{id}", "{id:[0-9]+}") or a param embedded with literal
// text ("{name}.txt", "user-{id}", "{a}.{b}"). Any such segment matches exactly
// one path segment, because slash-spanning params are excluded upstream (see
// exactPathTemplate) and routed to the fallback list. It is therefore indexed as
// a single parameter slot in the trie, and mux's Route.Match enforces the exact
// intra-segment pattern — the trie only needs to produce the route as a
// candidate, never to decide it.
func segmentHasParam(seg string) bool {
	return strings.Contains(seg, "{")
}
