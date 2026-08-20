package rbac

import (
	"sort"
	"strings"
)

// Segment specificity scores, compared segment by segment to order overlapping rules.
// Each level constrains the set of paths a segment can match more tightly than the one below it:
// a literal matches one string, a constrained variable matches one segment drawn from a regex,
// a free variable matches any one segment, and a catch-all matches any number of them.
const (
	segCatchAll    = iota + 1 // {path:.*} - matches any number of segments
	segVariable               // {id} - matches exactly one segment, unconstrained
	segConstrained            // {id:[0-9]+} - matches exactly one segment, and only some of them
	segLiteral                // users - matches itself
)

// endpointRule is one (path, method) pair from the config, pre-scored so that overlapping
// rules resolve deterministically. Rules are sorted once at load time.
type endpointRule struct {
	endpoint *EndpointMapping

	// pattern is the endpoint path, which may contain mux variables.
	pattern string

	// method is the upper-cased declared method, or "*" for all methods.
	method string

	// isPublic mirrors endpoint.Public.
	isPublic bool

	// pathScore is the per-segment specificity of pattern.
	pathScore []int

	// methodScore is 1 for an explicitly declared method and 0 for "*", so that an explicit
	// method wins over a wildcard on an otherwise equally specific path.
	methodScore int
}

// matchesMethod reports whether the rule covers the given upper-cased request method.
// A rule declared with "*" covers every method, including methods GoFr does not know about
// (WebDAV verbs, for example). That is the safe direction here: because a rule imposes a
// permission requirement rather than granting access, covering an unknown verb tightens
// enforcement rather than loosening it.
func (r *endpointRule) matchesMethod(methodUpper string) bool {
	return matchesHTTPMethod(methodUpper, []string{r.method})
}

// matches reports whether the rule covers the given request.
func (r *endpointRule) matches(methodUpper, path string, config *Config) bool {
	return r.matchesMethod(methodUpper) && matchesEndpointPattern(r.endpoint, path, config)
}

// buildEndpointRules expands endpoints into one rule per declared method and orders them
// most-specific-first, so that resolution does not depend on declaration order.
//
// Duplicate (method, path) declarations collapse to the last one, matching how the lookup
// maps are built, so both resolution paths always agree.
func buildEndpointRules(endpoints []EndpointMapping) []endpointRule {
	byKey := make(map[string]endpointRule, len(endpoints))
	order := make([]string, 0, len(endpoints))

	for i := range endpoints {
		endpoint := &endpoints[i]

		methods := endpoint.Methods
		if len(methods) == 0 {
			methods = []string{"*"}
		}

		for _, method := range methods {
			methodUpper := strings.ToUpper(method)
			key := buildEndpointKey(endpoint, methodUpper)

			methodScore := 1
			if methodUpper == "*" {
				methodScore = 0
			}

			if _, seen := byKey[key]; !seen {
				order = append(order, key)
			}

			byKey[key] = endpointRule{
				endpoint:    endpoint,
				pattern:     endpoint.Path,
				method:      methodUpper,
				isPublic:    endpoint.Public,
				pathScore:   pathSpecificity(endpoint.Path),
				methodScore: methodScore,
			}
		}
	}

	rules := make([]endpointRule, 0, len(order))
	for _, key := range order {
		rules = append(rules, byKey[key])
	}

	sort.SliceStable(rules, func(i, j int) bool {
		if cmp := compareSpecificity(rules[i].pathScore, rules[j].pathScore); cmp != 0 {
			return cmp > 0
		}

		return rules[i].methodScore > rules[j].methodScore
	})

	return rules
}

// pathSpecificity scores each segment of a path pattern.
func pathSpecificity(pattern string) []int {
	if pattern == "" {
		return nil
	}

	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	scores := make([]int, 0, len(segments))

	for _, segment := range segments {
		switch {
		case !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}"):
			scores = append(scores, segLiteral)
		case isCatchAllVariable(segment):
			scores = append(scores, segCatchAll)
		case strings.Contains(segment, ":"):
			scores = append(scores, segConstrained)
		default:
			scores = append(scores, segVariable)
		}
	}

	return scores
}

// isCatchAllVariable reports whether a "{name:regex}" segment can span multiple path
// segments. Only the documented catch-all forms - ".*" and ".+" - are recognized as such;
// any other constraint is assumed to stay within one segment.
//
// The assumption is not always true. A constraint that admits "/" itself, such as
// "{path:[a-z/]+}", spans multiple segments but is scored as a single-segment variable, so a
// pattern using one can outrank a narrower pattern it fully contains and end up governing a
// request the narrower entry was written for. Recognizing those in general means parsing the
// constraint, which is not worth the machinery; write multi-segment matches as "{path:.*}"
// or "{path:.+}", which are the documented forms, and the ordering holds.
func isCatchAllVariable(segment string) bool {
	inner := segment[1 : len(segment)-1]

	idx := strings.Index(inner, ":")
	if idx < 0 {
		return false
	}

	switch strings.TrimSpace(inner[idx+1:]) {
	case ".*", ".+":
		return true
	default:
		return false
	}
}

// compareSpecificity orders two segment score vectors, most specific first.
// The first differing segment decides; if one vector is a prefix of the other, the longer
// (more constrained) pattern wins. Returns >0 when a is more specific than b.
func compareSpecificity(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] - b[i]
		}
	}

	return len(a) - len(b)
}

// resolveEndpoint returns the most specific rule covering the request, and whether it is public.
func resolveEndpoint(methodUpper, path string, rules []endpointRule, config *Config) (*EndpointMapping, bool) {
	for i := range rules {
		if rules[i].matches(methodUpper, path, config) {
			return rules[i].endpoint, rules[i].isPublic
		}
	}

	return nil, false
}

// resolve finds the endpoint governing a request, preferring the O(1) exact-path lookups.
//
// The exact lookups stay consistent with the ordered scan because a literal path is always
// more specific than any pattern, and an explicitly declared method is always preferred over
// a wildcard one - which is why the request's own method is probed before "*".
func (c *Config) resolve(methodUpper, path string) (*EndpointMapping, bool) {
	if endpoint, isPublic := c.getExactEndpoint(methodUpper + ":" + path); endpoint != nil {
		return endpoint, isPublic
	}

	if endpoint, isPublic := c.getExactEndpoint("*:" + path); endpoint != nil {
		return endpoint, isPublic
	}

	return resolveEndpoint(methodUpper, path, c.rules, c)
}
