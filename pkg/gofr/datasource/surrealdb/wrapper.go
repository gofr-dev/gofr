package surrealdb

import (
	"context"

	"github.com/surrealdb/surrealdb.go"
)

// DBWrapper wraps a *surrealdb.DB and implements the DB interface.
// This allows package-level generic functions to be used through the interface.
type DBWrapper struct {
	db *surrealdb.DB
}

// NewDBWrapper creates a new wrapper around a surrealdb.DB instance.
func NewDBWrapper(db *surrealdb.DB) *DBWrapper {
	return &DBWrapper{db: db}
}

// Use sets the namespace and database to use.
func (w *DBWrapper) Use(ctx context.Context, namespace, database string) error {
	return w.db.Use(ctx, namespace, database)
}

// SignIn authenticates a user.
func (w *DBWrapper) SignIn(ctx context.Context, auth *surrealdb.Auth) (string, error) {
	return w.db.SignIn(ctx, auth)
}

// Info retrieves information about the current session.
func (w *DBWrapper) Info(ctx context.Context) (any, error) {
	return w.db.Info(ctx)
}

// GetDB returns the underlying *surrealdb.DB for package-level operations.
// This is used internally for Query, Select, Create, Update, Insert, Delete operations.
func (w *DBWrapper) GetDB() *surrealdb.DB {
	return w.db
}
