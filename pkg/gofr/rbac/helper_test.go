package rbac

import (
	"context"
	"testing"

	"gofr.dev/pkg/gofr"
)

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
			// Create base context with the userRole value
			baseCtx := context.WithValue(t.Context(), userRole, tt.ctxRoleVal)

			// Wrap baseCtx in gofr.Context
			gofrCtx := &gofr.Context{Context: baseCtx}

			got := HasRole(gofrCtx, tt.checkRole)
			if got != tt.expectedRes {
				t.Errorf("HasRole() = %v, want %v", got, tt.expectedRes)
			}
		})
	}
}
func TestGetUserRole(t *testing.T) {
	expectedRole := "editor"
	baseCtx := context.WithValue(t.Context(), userRole, expectedRole)
	gofrCtx := &gofr.Context{Context: baseCtx}

	if role := GetUserRole(gofrCtx); role != expectedRole {
		t.Errorf("GetUserRole() = %v, want %v", role, expectedRole)
	}

	// Test no role set should return ""
	emptyCtx := &gofr.Context{Context: t.Context()}
	if role := GetUserRole(emptyCtx); role != "" {
		t.Errorf("GetUserRole() with no role = %v, want empty string", role)
	}
}
