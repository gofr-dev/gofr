package rbac

import (
	"encoding/json"
	"os"

	"gofr.dev/pkg/gofr"
)

type Config struct {
	RoleWithPermissions map[string][]string `json:"roles"` // Role: [Allowed routes]
	RoleExtractorFunc   func(ctx *gofr.Context) ([]string, error)
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

/* sample .json
{
  "roles": {
    "admin": ["*"],
    "editor": ["/posts/*", "/dashboard"],
    "user": ["/profile", "/home"]
  }
}
*/
