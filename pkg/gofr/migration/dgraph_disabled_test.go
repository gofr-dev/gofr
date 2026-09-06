//go:build gofr_nodgraph

package migration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A build made with -tags gofr_nodgraph must refuse a Dgraph migration rather
// than skip it: Run treats a checkAndCreateMigrationTable error as fatal, which
// is what stops a migration being recorded as applied when it was not.

func TestDgraphMigratorOmitted_RefusesToMigrate(t *testing.T) {
	dm := dgraphDS{}.apply(nil)

	require.ErrorIs(t, dm.checkAndCreateMigrationTable(nil), errDgraphMigratorOmitted)

	_, err := dm.getLastMigration(nil)
	require.ErrorIs(t, err, errDgraphMigratorOmitted)

	require.ErrorIs(t, dm.commitMigration(nil, transactionData{}), errDgraphMigratorOmitted)
}

func TestDgraphMigratorOmitted_DataSourceMethodsError(t *testing.T) {
	ds := dgraphDS{}

	require.ErrorIs(t, ds.ApplySchema(context.Background(), "schema"), errDgraphMigratorOmitted)
	require.ErrorIs(t, ds.AddOrUpdateField(context.Background(), "f", "string", ""), errDgraphMigratorOmitted)
	require.ErrorIs(t, ds.DropField(context.Background(), "f"), errDgraphMigratorOmitted)
}

func TestDgraphMigratorOmitted_NamesItself(t *testing.T) {
	assert.Equal(t, "DGraph (omitted)", dgraphMigrator{}.name())
}
