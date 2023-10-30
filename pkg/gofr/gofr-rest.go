package gofr

// RestReader is an interface for reading resources in a RESTful context.
type RestReader interface {
	// Read retrieves a resource using the provided *Context and returns the resource and an error.
	Read(c *Context) (interface{}, error)
}

// RestIndexer is an interface for listing resources in a RESTful context.
type RestIndexer interface {
	// Index retrieves a list of resources using the provided *Context and returns the list and an error.
	Index(c *Context) (interface{}, error)
}

// RestCreator is an interface for creating resources in a RESTful context.
type RestCreator interface {
	// Create creates a new resource using the provided *Context and returns the created resource and an error.
	Create(c *Context) (interface{}, error)
}

// RestUpdater is an interface for updating resources in a RESTful context.
type RestUpdater interface {
	// Update updates an existing resource using the provided *Context and returns the updated resource and an error.
	Update(c *Context) (interface{}, error)
}

// RestDeleter is an interface for deleting resources in a RESTful context.
type RestDeleter interface {
	// Delete removes a resource using the provided *Context and returns an interface and error.
	Delete(c *Context) (interface{}, error)
}

// RestPatcher is an interface for making partial updates to resources in a RESTful context.
type RestPatcher interface {
	// Patch applies partial updates to a resource using the provided *Context and returns the updated resource and an error.
	Patch(c *Context) (interface{}, error)
}

// REST method adds REST-like routes to the application based on the provided entity name and handler interfaces.
//
// It examines the provided handler for interface implementations and automatically adds routes for the
// standard HTTP methods (GET, POST, PUT, DELETE, PATCH) associated with the RESTful operations:
// - RestIndexer for listing (GET)
// - RestReader for retrieving (GET)
// - RestCreator for creating (POST)
// - RestUpdater for updating (PUT)
// - RestDeleter for deleting (DELETE)
// - RestPatcher for partial updates (PATCH)
//
// If the handler implements any of these interfaces, the corresponding routes are created with paths
// that include the specified entity name.
//
// Example usage:
//
//	app.REST("users", &UserHandler{})
//
// This will automatically generate routes for listing, retrieving, creating, updating, deleting, and patching
// user entities using the provided handler (UserHandler).
func (g *Gofr) REST(entity string, handler interface{}) {
	if c, ok := handler.(RestIndexer); ok {
		g.GET("/"+entity, c.Index)
	}

	if c, ok := handler.(RestReader); ok {
		g.GET("/"+entity+"/{id}", c.Read)
	}

	if c, ok := handler.(RestCreator); ok {
		g.POST("/"+entity, c.Create)
	}

	if c, ok := handler.(RestDeleter); ok {
		g.DELETE("/"+entity+"/{id}", c.Delete)
	}

	if c, ok := handler.(RestUpdater); ok {
		g.PUT("/"+entity+"/{id}", c.Update)
	}

	if c, ok := handler.(RestPatcher); ok {
		g.PATCH("/"+entity+"/{id}", c.Patch)
	}
}
