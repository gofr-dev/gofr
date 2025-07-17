package rbac

import (
	"encoding/json"
	"net/http"
	"os"
)

type Config struct {
	RoleWithPermissions map[string][]string `json:"roles"` // Role: [Allowed routes]
	RoleExtractorFunc   func(req *http.Request, args ...any) (string, error)
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

/* is the below sample .json file as expected?
{
 "roles": {
   "admin": ["*"],
   "editor": ["/posts/*", "/dashboard"],
   "user": ["/profile", "/home"]
 }
}
*/

// for the route specific
// should we validate the request with the role or add that role permission to that route? and should it work independently without rbac.usemiddleware?

/*
given in problem statement
app.Use(rbac.Middleware(config))  // Global
app.GET("/admin", rbac.RequireRole("admin")(handler)) // Route-specific




implemented
app.UseMiddleware(rbac.Middleware(rbacConfigs))
app.GET("/greet", rbac.RequireRole("admin", handler))


*/
