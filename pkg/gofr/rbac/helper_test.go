package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockContextValueGetter implements ContextValueGetter interface for testing.
type mockContextValueGetter struct {
	value func(key any) any
}

func (m *mockContextValueGetter) Value(key any) any {
	if m.value != nil {
		return m.value(key)
	}

	return nil
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name        string
		ctxRoleVal  string
		checkRole   string
		expectedRes bool
	}{
		{"matching role", "admin", "admin", true},
		{"non-matching role", "viewer", "admin", false},
		{"empty role in context", "", "admin", false},
		{"nil role in context", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &mockContextValueGetter{
				value: func(key any) any {
					if key == userRole {
						return tt.ctxRoleVal
					}
					return nil
				},
			}

			got := HasRole(ctx, tt.checkRole)
			assert.Equal(t, tt.expectedRes, got)
		})
	}
}

func TestGetUserRole(t *testing.T) {
	expectedRole := "editor"
	ctx := &mockContextValueGetter{
		value: func(key any) any {
			if key == userRole {
				return expectedRole
			}
			return nil
		},
	}

	role := GetUserRole(ctx)
	assert.Equal(t, expectedRole, role)

	// Test no role set should return ""
	emptyCtx := &mockContextValueGetter{
		value: func(_ any) any {
			return nil
		},
	}
	role = GetUserRole(emptyCtx)
	assert.Empty(t, role)
}

func TestHasRoleFromContext(t *testing.T) {
	tests := []struct {
		name        string
		ctxRoleVal  string
		checkRole   string
		expectedRes bool
	}{
		{"matching role", "admin", "admin", true},
		{"non-matching role", "viewer", "admin", false},
		{"empty role in context", "", "admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), userRole, tt.ctxRoleVal)
			got := HasRoleFromContext(ctx, tt.checkRole)
			assert.Equal(t, tt.expectedRes, got)
		})
	}
}

func TestGetUserRoleFromContext(t *testing.T) {
	expectedRole := "editor"
	ctx := context.WithValue(context.Background(), userRole, expectedRole)

	role := GetUserRoleFromContext(ctx)
	assert.Equal(t, expectedRole, role)

	// Test no role set should return ""
	emptyCtx := context.Background()
	role = GetUserRoleFromContext(emptyCtx)
	assert.Empty(t, role)
}

func TestHasRole_NilContext(t *testing.T) {
	got := HasRole(nil, "admin")
	assert.False(t, got)
}

func TestGetUserRole_NilContext(t *testing.T) {
	role := GetUserRole(nil)
	assert.Empty(t, role)
}
