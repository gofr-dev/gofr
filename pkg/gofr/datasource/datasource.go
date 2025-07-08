/*
Package datasource provides a way to add external datasources to the application.
A datasource is a component that provides access to data, such as a database or message queue.
The core framework includes built-in support for SQL and Redis datasources.
*/
package datasource

import "gofr.dev/pkg/gofr/config"

type Datasource interface {
	Register(config config.Config)
}

// Question is: is container aware exactly "Redis" is there or some opaque datasource. in the later case, how do we
// retrieve from context?
