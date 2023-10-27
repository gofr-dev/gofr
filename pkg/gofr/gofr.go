// Package gofr provides a comprehensive framework for building web applications following RESTful principles.
//
// GoFr simplifies the process of creating RESTful APIs and web services by offering a set of convenient
// features and abstractions for routing, handling HTTP requests, and managing resources.
//
// Key Features:
// - Automatic RESTful route generation based on provided handler interfaces.
// - Middleware support for request processing and authentication.
// - Configurable error handling and response formatting.
// - Integration with common data storage technologies.
// - Structured application architecture with clear separation of concerns.
//
// GoFr is designed to help developers quickly create robust and well-structured web applications that adhere
// to RESTful best practices. It is suitable for a wide range of use cases, from small projects to larger,
// production-grade applications.
package gofr

import (
	"net/http"
	"strings"

	"gofr.dev/pkg/datastore"

	"gofr.dev/pkg/gofr/metrics"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/notifier"
)

// Gofr is a struct that holds essential information about an application built with the GoFr framework.
//
// This type encompasses various aspects of an application, including data store connection information,
// configuration, server details, template directories, logging, metrics, notifications, resource mapping,
// custom resource shapes, service health checks, and database health checks.
//
// When initializing a new GoFr application, an instance of the Gofr type is typically created to manage
// and provide access to these components and configurations. It serves as a central structure for the
// application's core functionality and integration with various services and tools.
type Gofr struct {
	// Datastore represents the data store component used by the application.
	//
	// It encapsulates various data storage technologies, including SQL, NoSQL databases, and pub/sub systems.
	// This field provides a unified interface for data storage and retrieval in the application.
	datastore.DataStore

	cmd *cmdApp
	// Config is the config implementation that can be used to retrieve configurations.
	Config Config
	// Server holds server-related information, including port, routes, metrics port, etc.
	Server *server
	// TemplateDir denotes the path to the static files directory that needs to be rendered.
	TemplateDir string
	// Logger is the logger used to send logs to the output.
	Logger log.Logger
	// Metric is the metrics implementation that can be used to define custom metrics.
	Metric metrics.Metric
	// Notifier handles the publishing and subscribing of notifications.

	Notifier notifier.Notifier

	// ResourceMap is the default projections based on the resources that the application provides.
	ResourceMap map[string][]string
	// ResourceCustomShapes is the custom projections that users can define for the resources the application provides.
	ResourceCustomShapes map[string][]string

	// ServiceHealth is the health check data about the services connected to the application.
	ServiceHealth []HealthCheck
	// DatabaseHealth is the health check data about the databases connected to the application.
	DatabaseHealth []HealthCheck
}

// Start initiates the execution of the application. It checks if there is a command (cmd) associated with the Gofr instance.
// If a command is present, it calls the Start method of the command, passing the logger as a parameter.
// If no command is available, it starts the server by calling its Start method, also passing the logger.
// This method effectively launches the application, handling both command-line and server-based execution scenarios.
func (g *Gofr) Start() {
	if g.cmd != nil {
		g.cmd.Start(g.Logger)
	} else {
		g.Server.Start(g.Logger)
	}
}

func (g *Gofr) addRoute(method, path string, handler Handler) {
	if g.cmd != nil {
		g.cmd.Router.AddRoute(path, handler) // Ignoring method in CMD App.
	} else {
		if path != "/" {
			path = strings.TrimSuffix(path, "/")
			g.Server.Router.Route(method, path+"/", handler)
		}
		g.Server.Router.Route(method, path, handler)
	}
}

// GET adds a route for handling HTTP GET requests.
func (g *Gofr) GET(path string, handler Handler) {
	g.addRoute(http.MethodGet, path, handler)
}

// PUT adds a route for handling HTTP PUT requests.
func (g *Gofr) PUT(path string, handler Handler) {
	g.addRoute(http.MethodPut, path, handler)
}

// POST adds a route for handling HTTP POST requests.
func (g *Gofr) POST(path string, handler Handler) {
	g.addRoute(http.MethodPost, path, handler)
}

// DELETE adds a route for handling HTTP DELETE requests.
func (g *Gofr) DELETE(path string, handler Handler) {
	g.addRoute(http.MethodDelete, path, handler)
}

// PATCH adds a route for handling HTTP PATCH requests.
func (g *Gofr) PATCH(path string, handler Handler) {
	g.addRoute(http.MethodPatch, path, handler)
}

// Deprecated: EnableSwaggerUI is deprecated. Auto enabled swagger-endpoints.
func (g *Gofr) EnableSwaggerUI() {
	g.addRoute(http.MethodGet, "/swagger", SwaggerUIHandler)
	g.addRoute(http.MethodGet, "/swagger/{name}", SwaggerUIHandler)

	g.Logger.Warn("Usage of EnableSwaggerUI is deprecated. Swagger Endpoints are auto-enabled")
}
