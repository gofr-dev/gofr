package rbac

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/pkg/gofr"
)

var errNoRole = errors.New("no role")

// mockHandler just writes "OK" to the ResponseWriter.
func mockHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = io.WriteString(w, "OK")
}

func TestMiddleware_UnauthorizedRoleExtractor(t *testing.T) {
	cfg := &Config{
		RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
			return "", errNoRole
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/some", http.NoBody)
	rr := httptest.NewRecorder()

	h := Middleware(cfg)(http.HandlerFunc(mockHandler))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	if body := rr.Body.String(); body == "" {
		t.Errorf("expected error message in body, got empty")
	}
}

func TestMiddleware_ForbiddenPath(t *testing.T) {
	cfg := &Config{
		RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
			return "user", nil
		},
		RoleWithPermissions: map[string][]string{
			"user": {},
		},
		OverRides: map[string]bool{},
	}

	req := httptest.NewRequest(http.MethodGet, "/forbidden", http.NoBody)
	rr := httptest.NewRecorder()

	h := Middleware(cfg)(http.HandlerFunc(mockHandler))
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestMiddleware_SuccessAddsRole(t *testing.T) {
	cfg := &Config{
		RoleExtractorFunc: func(_ *http.Request, _ ...any) (string, error) {
			return "admin", nil
		},
		RoleWithPermissions: map[string][]string{
			"admin": {"*"},
		},
	}

	var ctxRole string

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract role from context to verify it's stored
		if val, ok := r.Context().Value(userRole).(string); ok {
			ctxRole = val
		}

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/anything", http.NoBody)
	rr := httptest.NewRecorder()

	h := Middleware(cfg)(nextHandler)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}

	if ctxRole != "admin" {
		t.Errorf("expected role 'admin' in context, got '%s'", ctxRole)
	}
}

func TestRequireRole_Match(t *testing.T) {
	expectedResult := "success"

	handler := func(_ *gofr.Context) (any, error) {
		return expectedResult, nil
	}

	// Create a context with the expected role
	baseCtx := context.WithValue(t.Context(), userRole, "admin")
	gCtx := &gofr.Context{Context: baseCtx}

	got, err := RequireRole("admin", handler)(gCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != expectedResult {
		t.Errorf("got %v, want %v", got, expectedResult)
	}
}

func TestRequireRole_NoMatch(t *testing.T) {
	handler := func(_ *gofr.Context) (any, error) {
		return "should not run", nil
	}

	// context with wrong role
	baseCtx := context.WithValue(t.Context(), userRole, "user")
	gCtx := &gofr.Context{Context: baseCtx}

	_, err := RequireRole("admin", handler)(gCtx)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err.Error() != "forbidden: access denied" {
		t.Errorf("got error %q, want %q", err.Error(), "forbidden: access denied")
	}
}
