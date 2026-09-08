//go:build gofr_nodgraph

package migration

import (
	"context"
	"errors"

	"gofr.dev/pkg/gofr/container"
)

// Stubs for a build made with -tags gofr_nodgraph.
//
// The Dgraph migrator is the only thing in GoFr that imports
// github.com/dgraph-io/dgo, and it does so for a single value: the *api.Mutation
// that commitMigration hands to DGraph.Mutate, whose parameter is any. That one
// construction pulls in dgo's protobufs, which pull in google.golang.org/grpc --
// 83 packages and 3.7 MB for a service that has never heard of Dgraph.
//
// The container's Dgraph interface itself is dgo-free (its methods take and
// return any), so nothing else in the default path is affected, and a service
// that does use Dgraph links the driver through its own datasource module.
//
// Under the tag a Dgraph migration fails rather than silently doing nothing:
// running migrations against a datasource whose bookkeeping table cannot be
// written is how you get a migration applied twice.
type dgraphDS struct {
	client DGraph
}

type dgraphMigrator struct {
	dgraphDS
	migrator
}

var errDgraphMigratorOmitted = errors.New("dgraph migrations are unavailable: this binary was built with " +
	"-tags gofr_nodgraph, which omits the Dgraph migrator. Rebuild without the tag to run them")

func (ds dgraphDS) apply(m migrator) migrator {
	return dgraphMigrator{dgraphDS: ds, migrator: m}
}

func (dgraphDS) ApplySchema(context.Context, string) error { return errDgraphMigratorOmitted }

func (dgraphDS) AddOrUpdateField(context.Context, string, string, string) error {
	return errDgraphMigratorOmitted
}

func (dgraphDS) DropField(context.Context, string) error { return errDgraphMigratorOmitted }

func (dgraphMigrator) checkAndCreateMigrationTable(*container.Container) error {
	return errDgraphMigratorOmitted
}

func (dgraphMigrator) getLastMigration(*container.Container) (int64, error) {
	return 0, errDgraphMigratorOmitted
}

func (dm dgraphMigrator) beginTransaction(c *container.Container) transactionData {
	return dm.migrator.beginTransaction(c)
}

func (dgraphMigrator) commitMigration(*container.Container, transactionData) error {
	return errDgraphMigratorOmitted
}

func (dgraphMigrator) rollback(*container.Container, transactionData) {}

func (dgraphMigrator) name() string { return "DGraph (omitted)" }
