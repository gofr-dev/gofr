package rbac

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// patternEndpoints builds a config body of `count` mux-pattern rules, none of which match the
// path used by the tests below, followed by one that does. Pattern rules are what force the
// ordered scan; a config of literal paths never reaches it.
func patternEndpoints(count int) []EndpointMapping {
	endpoints := make([]EndpointMapping, 0, count+1)

	for i := range count {
		endpoints = append(endpoints, EndpointMapping{
			Path:                fmt.Sprintf("/other%d/{id}/sub/{sub}", i),
			Methods:             []string{"*"},
			RequiredPermissions: []string{"other:read"},
		})
	}

	return append(endpoints, EndpointMapping{
		Path:                "/admin/{path:.*}",
		Methods:             []string{"*"},
		RequiredPermissions: []string{"admin:read"},
	})
}

// TestConfig_resolve_concurrent drives the resolver from many goroutines at once, which is the
// only way a server ever runs it. Under -race this fails before the fix: resolving a pattern
// appended a route to the config's shared mux.Router on every call.
func TestConfig_resolve_concurrent(t *testing.T) {
	const (
		goroutines = 50
		iterations = 20
	)

	config := newTestConfig(t, patternEndpoints(5), nil)

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			for range iterations {
				endpoint, isPublic := config.resolve(http.MethodDelete, "/admin/orgs/123")

				assert.False(t, isPublic)
				assert.NotNil(t, endpoint)
			}
		}()
	}

	wg.Wait()
}

// TestMiddleware_concurrent is the same check one layer up, through the exported entry point
// an application actually mounts.
func TestMiddleware_concurrent(t *testing.T) {
	const (
		goroutines = 50
		iterations = 20
	)

	config := newTestConfig(t, patternEndpoints(5),
		[]RoleDefinition{{Name: "admin", Permissions: []string{"admin:read"}}})

	handler := Middleware(config)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			for range iterations {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/admin/orgs/123", http.NoBody)
				req.Header.Set("X-User-Role", "admin")

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusOK, rec.Code)
			}
		}()
	}

	wg.Wait()
}

// TestConfig_resolve_allocations pins the per-request cost of the ordered scan to the size of the
// request rather than the size of the config. RBAC runs on every request of every route, so an
// allocation here lands on the whole application's hot path.
//
// The assertion is comparative rather than an absolute count: the scan allocates a small fixed set
// of per-request values, and exactly what that comes to varies with the Go version and with
// whether the race detector is on. What must not vary is that it stays flat as rules are added.
// Before the fix each rule's pattern was recompiled per request, so this ran to 1064, 3924 and
// 9684 allocations for the three sizes below.
func TestConfig_resolve_allocations(t *testing.T) {
	measure := func(rules int) float64 {
		config := newTestConfig(t, patternEndpoints(rules), nil)

		// Warm up, so any first-call lazy work is not counted.
		config.resolve(http.MethodDelete, "/admin/orgs/123")

		return testing.AllocsPerRun(100, func() {
			config.resolve(http.MethodDelete, "/admin/orgs/123")
		})
	}

	baseline := measure(5)

	// Generous: a scan that recompiles patterns costs hundreds of allocations per added rule, so
	// any real regression clears this by orders of magnitude.
	const tolerance = 4

	for _, rules := range []int{20, 50} {
		t.Run(fmt.Sprintf("%d_rules", rules+1), func(t *testing.T) {
			assert.LessOrEqual(t, measure(rules), baseline+tolerance,
				"resolve allocates per rule scanned; cost must not scale with config size")
		})
	}
}

func TestEndpointRule_matchesMethod(t *testing.T) {
	testCases := []struct {
		desc         string
		ruleMethod   string
		requestUpper string
		expected     bool
	}{
		{"explicit method matches itself", http.MethodGet, http.MethodGet, true},
		{"explicit method rejects another", http.MethodGet, http.MethodDelete, false},
		{"explicit method matches case-insensitively", http.MethodGet, "get", true},
		{"wildcard matches any known method", "*", http.MethodDelete, true},
		{"wildcard matches unknown method", "*", "PROPFIND", true},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rule := endpointRule{method: tc.ruleMethod}
			assert.Equal(t, tc.expected, rule.matchesMethod(tc.requestUpper))
		})
	}
}

func BenchmarkConfig_resolve(b *testing.B) {
	for _, rules := range []int{5, 20, 50} {
		b.Run(fmt.Sprintf("%d_rules", rules+1), func(b *testing.B) {
			config := newTestConfig(b, patternEndpoints(rules), nil)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				endpoint, _ := config.resolve(http.MethodDelete, "/admin/orgs/123")
				require.NotNil(b, endpoint)
			}
		})
	}
}
