package rbac

import (
	"encoding/json"
	"net/http"
	"os"
)

type Config struct {
	RouteWithPermissions map[string][]string `json:"route"` // route: [Allowed roles]
	RoleExtractorFunc    func(req *http.Request, args ...any) (string, error)
	OverRides            map[string]bool // route: [override bool]
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
