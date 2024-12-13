package mongo

import "errors"

var (
	// ErrInvalidURI is returned when the MongoDB URI is invalid or cannot be parsed.
	ErrInvalidURI = fmt.Errorf("%w: invalid MongoDB URI", ErrConnection)

	// ErrAuthentication is returned when authentication fails.
	ErrAuthentication = fmt.Errorf("%w: authentication failed", ErrConnection)

	// ErrDatabaseConnection is returned when the client fails to connect to the specified database.
	ErrDatabaseConnection = fmt.Errorf("%w: failed to connect to database", ErrConnection)

	// ErrGenericConnection is returned for general connection issues.
	ErrConnection = errors.New("MongoDB connection error")
)
