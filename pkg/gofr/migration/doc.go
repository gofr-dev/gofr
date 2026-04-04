// Package migration provides a framework for running versioned data migrations
// across the datasource backends supported by GoFr.
//
// It coordinates migration execution with distributed locking, transaction
// management where supported, and metadata tracking. Supported backends
// include SQL, Redis, MongoDB, ClickHouse, Cassandra, ArangoDB, DGraph,
// SurrealDB, ScyllaDB, Elasticsearch, OpenTSDB, and others.
//
// Migrations are registered as a map of version numbers to [Migrate]
// implementations and executed via [Run].
package migration
