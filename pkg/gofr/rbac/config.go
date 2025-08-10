package rbac

import (
	"encoding/json"
	"net/http"
	"os"
)

type Config struct {
	RoleWithPermissions map[string][]string `json:"roles"` // Role: [Allowed routes]
	RoleExtractorFunc   func(req *http.Request, args ...any) (string, error)
	OverRides           map[string]bool // role: [override bool]
}

func LoadPermissions(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
