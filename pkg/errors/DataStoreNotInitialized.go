package errors

import "fmt"

// DataStoreNotInitialized standard error for errors in initializing datastore
type DataStoreNotInitialized struct {
	DBName string
	Reason string
}

// Error returns an error message regarding the initialization of data store
func (d DataStoreNotInitialized) Error() string {
	return fmt.Sprintf("couldn't initialize %v, %v", d.DBName, d.Reason)
}
