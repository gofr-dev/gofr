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
// most-specific-first, so that a narrower rule governs a request even when a broader one is
// declared ahead of it. Rules that score identically - two patterns of the same shape, such as
// "/{a}/{b}" and "/{x}/{y}" - keep their declaration order, which is the one case where the
// order entries are written in still decides the outcome.
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

		if rules[i].methodScore != rules[j].methodScore {
			return rules[i].methodScore > rules[j].methodScore
		}

		// Nothing about the patterns separates them, so fall back on the safer outcome: a rule
		// that requires permissions outranks a public one. A tie is a config the operator did not
		// intend either way, and enforcing is the recoverable half of that mistake.
		return !rules[i].isPublic && rules[j].isPublic
	})

	return rules
}

// pathSpecificity scores each segment of a path pattern.
func pathSpecificity(pattern string) []int {
	if pattern == "" {
		return nil
	}

	segments := splitPatternSegments(strings.Trim(pattern, "/"))
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

// splitPatternSegments splits a path pattern on "/", ignoring separators that sit inside a
// "{name:regex}" constraint - "{path:[a-z/]+}" is one segment, not three. Splitting naively
// would shatter such a constraint into fragments that each look like a literal, scoring the
// loosest pattern in the config as the most specific one.
func splitPatternSegments(pattern string) []string {
	var (
		segments []string
		depth    int
		start    int
	)

	for i, r := range pattern {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '/':
			if depth == 0 {
				segments = append(segments, pattern[start:i])
				start = i + 1
			}
		}
	}

	return append(segments, pattern[start:])
}

// isCatchAllVariable reports whether a "{name:regex}" segment can span multiple path
// segments. Two forms qualify: the documented catch-alls ".*" and ".+", and any constraint
// that admits "/" itself, such as "{path:[a-z/]+}" - it matches "/files/a/b/c" just as a
// catch-all would, so scoring it as a single-segment variable would let the loosest pattern in
// the config outrank a narrower one it fully contains.
//
// Whether the constraint can *actually* produce a "/" is not decided here; a "/" appearing
// anywhere in it is enough. Deciding it properly means parsing the regex, and the conservative
// answer only ever moves a pattern down the ordering, which is the safe direction: it can lose
// to a narrower rule, never shadow one.
func isCatchAllVariable(segment string) bool {
	inner := segment[1 : len(segment)-1]

	idx := strings.Index(inner, ":")
	if idx < 0 {
		return false
	}

	constraint := strings.TrimSpace(inner[idx+1:])

	return constraint == ".*" || constraint == ".+" || strings.Contains(constraint, "/")
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
