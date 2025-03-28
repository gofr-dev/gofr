package migration

import (
	"context"
	"fmt"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"time"

	"gofr.dev/pkg/gofr/container"
)

// dgraphDS is the adapter struct that implements migration operations
type dgraphDS struct {
	client DGraph
}

// dgraphMigrator struct implements the migrator interface
type dgraphMigrator struct {
	DGraph
	migrator
}

const (
	dgraphSchema = `
		migrations.version: int @index(int) .
		migrations.method: string .
		migrations.start_time: datetime .
		migrations.duration: int .
		type Migration {
			version: int
			method: string
			start_time: datetime
			duration: int
		}
	`
	dgraphInsertMigrationMutation = `
		mutation {
			addMigration(input: [{
				version: %d,
				method: "%s",
				start_time: "%s",
				duration: %d
			}]) {
				migration {
					version
				}
			}
		}
	`
	getLastMigrationQuery = `
		{
			migrations(func: type(Migration), orderdesc: migrations.version, first: 1) {
				version: migrations.version
			}
		}
	`
)

// apply creates a new dgraphMigrator
func (ds dgraphDS) apply(m migrator) migrator {
	return dgraphMigrator{
		DGraph:   ds,
		migrator: m,
	}
}

func (ds dgraphDS) ApplySchema(ctx context.Context, schema string) error {
	return ds.client.ApplySchema(ctx, schema)
}

func (ds dgraphDS) AddOrUpdateField(ctx context.Context, fieldName, fieldType, directives string) error {
	return ds.client.AddOrUpdateField(ctx, fieldName, fieldType, directives)
}

func (ds dgraphDS) DropField(ctx context.Context, fieldName string) error {
	return ds.client.DropField(ctx, fieldName)
}

// checkAndCreateMigrationTable ensures migration schema exists
func (dm dgraphMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := dm.ApplySchema(context.Background(), dgraphSchema)
	if err != nil {
		c.Debug("Migration schema might already exist:", err)
	}

	return dm.migrator.checkAndCreateMigrationTable(c)
}

// getLastMigration retrieves the last applied migration version
func (dm dgraphMigrator) getLastMigration(c *container.Container) int64 {
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

	if resp != nil {
		return response.Migrations[0].Version
	}

	lm2 := dm.migrator.getLastMigration(c)
	if lm2 > 0 {
		return lm2
	}

	return 0
}

// beginTransaction starts a new migration transaction
func (dm dgraphMigrator) beginTransaction(c *container.Container) transactionData {
	data := dm.migrator.beginTransaction(c)

	c.Debug("Dgraph migrator begin successfully")

	return data
}

// commitMigration commits the migration and records its metadata
func (dm dgraphMigrator) commitMigration(c *container.Container, data transactionData) error {
	query := fmt.Sprintf(dgraphInsertMigrationMutation,
		data.MigrationNumber,
		"UP",
		data.StartTime.Format(time.RFC3339),
		time.Since(data.StartTime).Milliseconds(),
	)

	_, err := c.DGraph.Mutate(context.Background(), &api.Mutation{
		SetJson: []byte(query),
	})
	if err != nil {
		return err
	}

	c.Debugf("Inserted record for migration %v in Dgraph migrations", data.MigrationNumber)

	return dm.migrator.commitMigration(c, data)
}

// rollback handles migration failure and rollback
func (dm dgraphMigrator) rollback(c *container.Container, data transactionData) {
	dm.migrator.rollback(c, data)

	c.Fatalf("Migration %v failed and rolled back", data.MigrationNumber)
}
