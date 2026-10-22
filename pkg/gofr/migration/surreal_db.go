package migration

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	return surrealMigrator{
		SurrealDB: s.client,
		migrator:  m,
	}
}

const (
	getSurrealDBGoFrMigrationVersions = `SELECT version FROM gofr_migrations;`
	insertSurrealDBGoFrMigrationRow   = `CREATE gofr_migrations SET version = $version, method = $method, ` +
		`start_time = $start_time, duration = $duration;`
)

func getMigrationTableQueries() []string {
	return []string{
		"DEFINE TABLE IF NOT EXISTS gofr_migrations SCHEMAFULL;",
		"DEFINE FIELD IF NOT EXISTS id ON gofr_migrations TYPE string;",
		"DEFINE FIELD IF NOT EXISTS version ON gofr_migrations TYPE number;",
		"DEFINE FIELD IF NOT EXISTS method ON gofr_migrations TYPE string;",
		"DEFINE FIELD IF NOT EXISTS start_time ON gofr_migrations TYPE datetime;",
		"DEFINE FIELD IF NOT EXISTS duration ON gofr_migrations TYPE number;",
		"DEFINE INDEX IF NOT EXISTS version_method ON gofr_migrations COLUMNS version, method UNIQUE;",
	}
}

func (s surrealMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	// Create migration table directly
	for _, q := range getMigrationTableQueries() {
		if _, err := s.SurrealDB.Query(context.Background(), q, nil); err != nil {
			return fmt.Errorf("%w: %s: %w", errExecuteQuery, q, err)
		}
	}

	return s.migrator.checkAndCreateMigrationTable(c)
}

// surrealVersionToInt64 converts the `version` field returned by SurrealDB into an int64.
// The SurrealDB driver decodes a `number` column into different Go types depending on the
// stored value and driver version (e.g. int or int64 or uint64 or float64 over CBOR
// responses), so we handle each numeric type explicitly. Unknown/absent values default to 0.
func surrealVersionToInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case uint64:
		if n > math.MaxInt64 {
			return 0
		}

		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func (s surrealMigrator) getAppliedMigrations(c *container.Container) (map[int64]struct{}, error) {
	result, err := s.SurrealDB.Query(context.Background(), getSurrealDBGoFrMigrationVersions, nil)
	if err != nil {
		return nil, fmt.Errorf("surrealdb: %w", err)
	}

	c.Debugf("surrealDB applied migrations fetched: %v", len(result))

	applied, err := s.migrator.getAppliedMigrations(c)
	if err != nil {
		return nil, err
	}

	for _, r := range result {
		if row, ok := r.(map[string]any); ok {
			applied[surrealVersionToInt64(row[migrationFieldVersion])] = struct{}{}
		}
	}

	return applied, nil
}

func (s surrealMigrator) beginTransaction(c *container.Container) transactionData {
	data := s.migrator.beginTransaction(c)

	c.Debug("surrealDB migrator begin successfully")

	return data
}

func (s surrealMigrator) commitMigration(c *container.Container, data *transactionData) error {
	if data.UsedDatasources[dsSurrealDB] {
		_, err := s.SurrealDB.Query(context.Background(), insertSurrealDBGoFrMigrationRow, map[string]any{
			migrationFieldVersion:   data.MigrationNumber,
			migrationFieldMethod:    migrationMethodUP,
			migrationFieldStartTime: data.StartTime,
			migrationFieldDuration:  time.Since(data.StartTime).Milliseconds(),
		})
		if err != nil {
			return err
		}

		c.Debugf("inserted record for migration %v in surrealDB gofr_migrations table", data.MigrationNumber)
	}

	return s.migrator.commitMigration(c, data)
}

func (s surrealMigrator) rollback(c *container.Container, data *transactionData) {
	s.migrator.rollback(c, data)

	c.Fatalf("migration %v failed and rolled back", data.MigrationNumber)
}

func (s surrealMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	return s.migrator.lock(ctx, cancel, c, ownerID)
}

func (s surrealMigrator) unlock(c *container.Container, ownerID string) error {
	return s.migrator.unlock(c, ownerID)
}

func (surrealMigrator) name() string {
	return "SurrealDB"
}
