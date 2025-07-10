/*
Package datasource contains all the supported data sources in GoFr.
A datasource refers to any component that provides access to data — such as databases or message queues.
GoFr comes with built-in support for SQL and Redis data sources out of the box.
*/
package datasource

import "gofr.dev/pkg/gofr/config"

type Datasource interface {
	Register(config config.Config)
}

// Question is: is container aware exactly "Redis" is there or some opaque datasource. in the later case, how do we
// retrieve from context?
