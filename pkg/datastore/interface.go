package datastore

type tester interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}
