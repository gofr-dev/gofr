package migration

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dgraph-io/dgo/v210/protos/api"

	"gofr.dev/pkg/gofr/container"
)

const (
	// dgraphSchema defines the migration schema with fully qualified predicate names.
	dgraphSchema = `
		migrations.version: int @index(int) .
		migrations.method: string .
		migrations.start_time: datetime .
		migrations.duration: int .
		type Migration {
			migrations.version
			migrations.method
			migrations.start_time
			migrations.duration
		}
	`

	// getLastMigrationQuery fetches the most recent migration version.
	getLastMigrationQuery = `
		{
			migrations(func: type(Migration), orderdesc: migrations.version, first: 1) {
				migrations.version
			}
		}
	`
)

// dgraphDS is the adapter struct that implements migration operations.
type dgraphDS struct {
	client DGraph
}

// dgraphMigrator struct implements the migrator interface.
type dgraphMigrator struct {
	dgraphDS
	migrator
}

// apply creates a new dgraphMigrator.
func (ds dgraphDS) apply(m migrator) migrator {
	return &dgraphMigrator{
		dgraphDS: ds,
		migrator: m,
	}
}

// ApplySchema applies the given schema to DGraph. It takes a context and schema string as parameters
// and returns an error if the schema application fails.
func (ds dgraphDS) ApplySchema(ctx context.Context, schema string) error {
	return ds.client.ApplySchema(ctx, schema)
}

// AddOrUpdateField adds a new field or updates an existing field in DGraph schema.
// Parameters:
//   - ctx: The context for the operation
//   - fieldName: Name of the field to add or update
//   - fieldType: Data type of the field
//   - directives: Additional DGraph directives for the field
//
// Returns an error if the operation fails.
func (ds dgraphDS) AddOrUpdateField(ctx context.Context, fieldName, fieldType, directives string) error {
	return ds.client.AddOrUpdateField(ctx, fieldName, fieldType, directives)
}

// DropField removes a field from DGraph schema.
// Parameters:
//   - ctx: The context for the operation
//   - fieldName: Name of the field to remove
//
// Returns an error if the field deletion fails.
func (ds dgraphDS) DropField(ctx context.Context, fieldName string) error {
	return ds.client.DropField(ctx, fieldName)
}

// checkAndCreateMigrationTable ensures migration schema exists.
func (dm *dgraphMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := dm.ApplySchema(context.Background(), dgraphSchema)
	if err != nil {
		c.Debug("Migration schema might already exist:", err)
	}

	return dm.migrator.checkAndCreateMigrationTable(c)
}

// getLastMigration retrieves the last applied migration version.
func (dm *dgraphMigrator) getLastMigration(c *container.Container) int64 {
	var response struct {
		Migrations []struct {
			Version int64 `json:"version"`
		} `json:"migrations"`
	}

	resp, err := c.DGraph.Query(context.Background(), getLastMigrationQuery)
	if err != nil {
		c.Debug("Error fetching last migration:", err)
		return 0
	}

	// If a response is returned, marshal it to JSON bytes then unmarshal into our response struct.
	if resp != nil {
		b, err := json.Marshal(resp)
		if err != nil {
			c.Debug("Error marshaling response:", err)
			return 0
		}

		if err := json.Unmarshal(b, &response); err != nil {
			c.Debug("Error unmarshalling migration response:", err)
			return 0
		}

		if len(response.Migrations) > 0 {
			return response.Migrations[0].Version
		}
	}

	lm2 := dm.migrator.getLastMigration(c)
	if lm2 > 0 {
		return lm2
	}

	return 0
}

// beginTransaction starts a new migration transaction.
func (dm *dgraphMigrator) beginTransaction(c *container.Container) transactionData {
	data := dm.migrator.beginTransaction(c)

	c.Debug("Dgraph migrator begin successfully")

	return data
}

// commitMigration commits the migration and records its metadata.
func (dm *dgraphMigrator) commitMigration(c *container.Container, data transactionData) error {
	// Build the JSON payload for the migration record.
	payload := map[string]any{
		"migrations": []map[string]any{
			{
				"migrations.version":    data.MigrationNumber,
				"migrations.method":     "UP",
				"migrations.start_time": data.StartTime.Format(time.RFC3339),
				"migrations.duration":   time.Since(data.StartTime).Milliseconds(),
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = c.DGraph.Mutate(context.Background(), &api.Mutation{
		SetJson: jsonPayload,
	})
	if err != nil {
		return err
	}

	c.Debugf("Inserted record for migration %v in Dgraph migrations", data.MigrationNumber)

	return dm.migrator.commitMigration(c, data)
}

// rollback handles migration failure and rollback.
func (dm *dgraphMigrator) rollback(c *container.Container, data transactionData) {
	dm.migrator.rollback(c, data)

	c.Fatalf("Migration %v failed and rolled back", data.MigrationNumber)
}

func (*dgraphMigrator) Lock(*container.Container) error {
	return nil
}

func (*dgraphMigrator) Unlock(*container.Container) error {
	return nil
}

func (dm *dgraphMigrator) Next() migrator {
	return dm.migrator
}

func (*dgraphMigrator) Name() string {
	return "DGraph"
}
