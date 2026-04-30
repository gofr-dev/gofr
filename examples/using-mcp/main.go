// Example: a tiny GoFr service that, with MCP_ENABLED=true in the env,
// also exposes its HTTP routes as Model Context Protocol tools at
// /mcp. AI coding assistants (Claude Desktop, Cursor, Continue, etc.)
// can connect and call the routes the same way a normal HTTP client
// would — with the same auth, the same observability, and the same
// handler logic.
//
// The handlers below are intentionally vanilla. There is no MCP-aware
// code in this file. The only thing that turns MCP on is the
// MCP_ENABLED entry in configs/.env.
package main

import (
	"errors"
	"sync"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

type CreateUserRequest struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// In-memory store. A real service would use c.SQL / c.Redis / etc.
// the same way; the MCP layer would still see learned schemas without
// any extra wiring.
type store struct {
	mu    sync.Mutex
	next  int
	items map[string]User
}

func main() {
	app := gofr.New()
	s := &store{
		next:  1,
		items: map[string]User{
			"1": {ID: "1", Name: "Ada Lovelace", Role: "engineer"},
		},
	}

	app.GET("/users", s.list)
	app.GET("/users/{id}", s.get)
	app.POST("/users", s.create)

	app.Run()
}

// list returns every user. Reads an optional ?role= filter so the
// MCP layer learns that this route accepts a "role" query parameter.
func (s *store) list(c *gofr.Context) (any, error) {
	role := c.Param("role")

	s.mu.Lock()
	defer s.mu.Unlock()

	users := make([]User, 0, len(s.items))
	for _, u := range s.items {
		if role != "" && u.Role != role {
			continue
		}

		users = append(users, u)
	}

	return users, nil
}

// get returns one user by id. The path parameter is automatically a
// required field in the resulting MCP tool input schema.
func (s *store) get(c *gofr.Context) (any, error) {
	id := c.PathParam("id")

	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.items[id]
	if !ok {
		return nil, http.ErrorEntityNotFound{Name: "user", Value: id}
	}

	return u, nil
}

// create binds a JSON body. The first time this handler runs the MCP
// learner records CreateUserRequest's JSON shape; from then on the
// "post_users" tool's input schema includes a typed body.
func (s *store) create(c *gofr.Context) (any, error) {
	var body CreateUserRequest
	if err := c.Bind(&body); err != nil {
		return nil, err
	}

	if body.Name == "" {
		return nil, errors.New("name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := nextID(s)

	u := User{ID: id, Name: body.Name, Role: body.Role}
	s.items[id] = u

	return u, nil
}

func nextID(s *store) string {
	s.next++
	return formatID(s.next)
}

// formatID is a separate function so it's easy to swap for a real
// id generator (uuid, snowflake) without touching the handler.
func formatID(n int) string {
	const digits = "0123456789"

	if n == 0 {
		return "0"
	}

	var buf [20]byte

	i := len(buf)

	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}

	return string(buf[i:])
}
