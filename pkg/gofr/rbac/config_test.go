package rbac

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Helper function to create a temporary JSON file for testing.
func createTempJSONFile(t *testing.T, data any) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "test.json")

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("error marshaling: %v", err)
	}

	if err := os.WriteFile(file, jsonBytes, 0600); err != nil {
		t.Fatalf("error writing file: %v", err)
	}

	return file
}

func TestLoadPermissions_Success(t *testing.T) {
	expected := Config{
		RoleWithPermissions: map[string][]string{
			"admin":  {"/admin", "/dashboard"},
			"viewer": {"/dashboard"},
		},
	}
	file := createTempJSONFile(t, struct {
		RoleWithPermissions map[string][]string `json:"roles"`
	}{
		RoleWithPermissions: expected.RoleWithPermissions,
	})

	got, err := LoadPermissions(file)

	if err != nil {
		t.Fatalf("LoadPermissions returned error: %v", err)
	}

	if !reflect.DeepEqual(got.RoleWithPermissions, expected.RoleWithPermissions) {
		t.Errorf("RoleWithPermissions mismatch: got %v, want %v", got.RoleWithPermissions, expected.RoleWithPermissions)
	}

	if !reflect.DeepEqual(got.OverRides, expected.OverRides) {
		t.Errorf("OverRides mismatch: got %v, want %v", got.OverRides, expected.OverRides)
	}
}

func TestLoadPermissions_FileNotFound(t *testing.T) {
	// Act
	_, err := LoadPermissions("nonexistentpath.json")
	// Assert
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

func TestLoadPermissions_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.json")

	// Write invalid JSON
	if err := os.WriteFile(file, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("could not write test file: %v", err)
	}

	_, err := LoadPermissions(file)
	if err == nil {
		t.Fatalf("expected JSON unmarshal error, got nil")
	}
}
