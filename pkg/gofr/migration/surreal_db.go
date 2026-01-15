package migration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/container"
)

var errExecuteQuery = errors.New("failed to execute migration query")

type surrealDS struct {
	client SurrealDB
}

func (s surrealDS) Query(ctx context.Context, query string, vars map[string]any) ([]any, error) {
	return s.client.Query(ctx, query, vars)
}

func (s surrealDS) CreateNamespace(ctx context.Context, namespace string) error {
	return s.client.CreateNamespace(ctx, namespace)
}

func (s surrealDS) CreateDatabase(ctx context.Context, database string) error {
	return s.client.CreateDatabase(ctx, database)
}

func (s surrealDS) DropNamespace(ctx context.Context, namespace string) error {
	return s.client.DropNamespace(ctx, namespace)
}

func (s surrealDS) DropDatabase(ctx context.Context, database string) error {
	return s.client.DropDatabase(ctx, database)
}

type surrealMigrator struct {
	SurrealDB
	migrator
}

func (s surrealDS) apply(m migrator) migrator {
	return &surrealMigrator{
		SurrealDB: s.client,
		migrator:  m,
	}
}

const (
	getLastSurrealDBGoFrMigration   = `SELECT version FROM gofr_migrations ORDER BY version DESC LIMIT 1;`
	insertSurrealDBGoFrMigrationRow = `CREATE gofr_migrations SET version = $version, method = $method, ` +
		`start_time = $start_time, duration = $duration;`
)

func getMigrationTableQueries() []string {
	return []string{
		"DEFINE TABLE gofr_migrations SCHEMAFULL;",
		"DEFINE FIELD id ON gofr_migrations TYPE string;",
		"DEFINE FIELD version ON gofr_migrations TYPE number;",
		"DEFINE FIELD method ON gofr_migrations TYPE string;",
		"DEFINE FIELD start_time ON gofr_migrations TYPE datetime;",
		"DEFINE FIELD duration ON gofr_migrations TYPE number;",
		"DEFINE INDEX version_method ON gofr_migrations COLUMNS version, method UNIQUE;",
	}
}

func (s *surrealMigrator) checkAndCreateMigrationTable(*container.Container) error {
	if _, err := s.SurrealDB.Query(context.Background(), "USE NS test DB test", nil); err != nil {
		return err
	}

	// Create migration table directly
	for _, q := range getMigrationTableQueries() {
		if _, err := s.SurrealDB.Query(context.Background(), q, nil); err != nil {
			return fmt.Errorf("%w: %s: %w", errExecuteQuery, q, err)
		}
	}

	return nil
}

func (s *surrealMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigration int64 // Default to 0 if no migrations found

	result, err := s.SurrealDB.Query(context.Background(), getLastSurrealDBGoFrMigration, nil)
	if err != nil {
		return 0
	}

	if len(result) > 0 {
		// Assuming the query returns a single row with the version
		if version, ok := result[0].(map[string]any)["version"].(float64); ok {
			lastMigration = int64(version)
		}
	}

	c.Debugf("surrealDB last migration fetched value is: %v", lastMigration)

	lm2 := s.migrator.getLastMigration(c)

	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (s *surrealMigrator) beginTransaction(c *container.Container) transactionData {
	data := s.migrator.beginTransaction(c)

	c.Debug("surrealDB migrator begin successfully")

	return data
}

func (s *surrealMigrator) commitMigration(c *container.Container, data transactionData) error {
	_, err := s.SurrealDB.Query(context.Background(), insertSurrealDBGoFrMigrationRow, map[string]any{
		"version":    data.MigrationNumber,
		"method":     "UP",
		"start_time": data.StartTime,
		"duration":   time.Since(data.StartTime).Milliseconds(),
	})
	if err != nil {
		return err
	}

	c.Debugf("inserted record for migration %v in surrealDB gofr_migrations table", data.MigrationNumber)

	return s.migrator.commitMigration(c, data)
}

func (s *surrealMigrator) rollback(c *container.Container, data transactionData) {
	s.migrator.rollback(c, data)

	c.Fatalf("migration %v failed and rolled back", data.MigrationNumber)
}

func (*surrealMigrator) Lock(*container.Container) error {
	return nil
}

func (*surrealMigrator) Unlock(*container.Container) error {
	return nil
}

func (*surrealMigrator) Name() string {
	return "SurrealDB"
}
