package mongo

import "errors"

var (
	// ErrInvalidURI is returned when the MongoDB URI is invalid or cannot be parsed
	ErrInvalidURI = errors.New("invalid MongoDB URI")

	// ErrAuthentication is returned when authentication fails
	ErrAuthentication = errors.New("authentication failed")

	// ErrDatabaseConnection is returned when the client fails to connect to the specified database
	ErrDatabaseConnection = errors.New("failed to connect to database")

	// ErrGenericConnection is returned for general connection issues
	ErrGenericConnection = errors.New("generic MongoDB connection error")
)
