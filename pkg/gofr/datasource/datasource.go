/*
Package datasource provides an interface for registering datasources with the gofr framework.
A datasource is a componentc that provides access to data, such as a database or a message queue.
The Datasource interface defines a method for registering a datasource with the gofr framework.
*/
package datasource

import "gofr.dev/pkg/gofr/config"

type Datasource interface {
	Register(config config.Config)
}

// Question is: is container aware exactly "Redis" is there or some opaque datasource. in the later case, how do we
// retrieve from context?
