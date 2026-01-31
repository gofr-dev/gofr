package migration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func initializeElasticsearchRunMocks(t *testing.T) (*MockElasticsearch, *container.Container) {
	t.Helper()

	mockElasticsearch := NewMockElasticsearch(gomock.NewController(t))

	mockContainer := container.NewContainer(nil)
	mockContainer.SQL = nil
	mockContainer.Redis = nil
	mockContainer.Mongo = nil
	mockContainer.Cassandra = nil
	mockContainer.PubSub = nil
	mockContainer.ArangoDB = nil
	mockContainer.SurrealDB = nil
	mockContainer.DGraph = nil
	mockContainer.Clickhouse = nil
	mockContainer.Oracle = nil
	mockContainer.OpenTSDB = nil
	mockContainer.ScyllaDB = nil
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)
	mockContainer.Elasticsearch = mockElasticsearch

	return mockElasticsearch, mockContainer
}

func TestMigrationRunElasticsearchMigrationFailureWhileCheckingTable(t *testing.T) {
	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	testutil.StderrOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			1: {UP: func(_ Datasource) error {
				return nil
			}},
		}

		// checkAndCreateMigrationTable: Check if index exists (error)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// checkAndCreateMigrationTable: Try to create index (fails)
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(assert.AnError)

		Run(migrationMap, mockContainer)
	})
}

func TestMigrationRunElasticsearchCurrentMigrationEqualLastMigration(t *testing.T) {
	migrationMap := map[int64]Migrate{
		1: {UP: func(_ Datasource) error {
			return nil
		}},
	}

	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	// checkAndCreateMigrationTable: Index already exists
	mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
		gomock.Any()).
		Return(map[string]any{
			"hits": map[string]any{
				"hits": []any{},
			},
		}, nil)

	// Pre-check: getLastMigration returns 1, so migration 1 is skipped
	mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
		getLastElasticsearchMigrationQuery()).
		Return(map[string]any{
			"hits": map[string]any{
				"hits": []any{
					map[string]any{
						"_source": map[string]any{
							"version": float64(1),
						},
					},
				},
			},
		}, nil)

	Run(migrationMap, mockContainer)
}

func TestElasticsearchMigrator_checkAndCreateMigrationTable_IndexExists(t *testing.T) {
	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	// Mock successful search (index exists)
	mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
		Return(map[string]any{
			"hits": map[string]any{
				"total": map[string]any{
					"value": float64(0),
				},
			},
		}, nil)

	ds := elasticsearchDS{client: mockElasticsearch}
	mg := elasticsearchMigrator{elasticsearchDS: ds, migrator: &Datasource{}}

	err := mg.checkAndCreateMigrationTable(mockContainer)
	assert.NoError(t, err)
}

func TestElasticsearchMigrator_getLastMigration_WithMigrations(t *testing.T) {
	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	// Mock successful search with existing migrations
	mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
		getLastElasticsearchMigrationQuery()).
		Return(map[string]any{
			"hits": map[string]any{
				"hits": []any{
					map[string]any{
						"_source": map[string]any{
							"version": float64(5),
						},
					},
				},
			},
		}, nil)

	ds := elasticsearchDS{client: mockElasticsearch}
	mg := elasticsearchMigrator{elasticsearchDS: ds, migrator: &Datasource{}}

	lastMigration, err := mg.getLastMigration(mockContainer)
	require.NoError(t, err)
	assert.Equal(t, int64(5), lastMigration)
}

func TestElasticsearchMigrator_getLastMigration_NoMigrations(t *testing.T) {
	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	// Mock successful search with no migrations
	mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
		getLastElasticsearchMigrationQuery()).
		Return(map[string]any{
			"hits": map[string]any{
				"hits": []any{},
			},
		}, nil)

	ds := elasticsearchDS{client: mockElasticsearch}
	mg := elasticsearchMigrator{elasticsearchDS: ds, migrator: &Datasource{}}

	lastMigration, err := mg.getLastMigration(mockContainer)
	require.NoError(t, err)
	assert.Equal(t, int64(0), lastMigration)
}

func TestElasticsearchMigrator_commitMigration_Success(t *testing.T) {
	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	// Mock successful document indexing
	mockElasticsearch.EXPECT().IndexDocument(gomock.Any(), elasticsearchMigrationIndex, "1", gomock.Any()).
		Return(nil)

	ds := elasticsearchDS{client: mockElasticsearch}
	mg := elasticsearchMigrator{elasticsearchDS: ds, migrator: &Datasource{}}

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
	}

	err := mg.commitMigration(mockContainer, data)
	assert.NoError(t, err)
}

func TestElasticsearchMigrator_commitMigration_Failure(t *testing.T) {
	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	// Mock failed document indexing
	mockElasticsearch.EXPECT().IndexDocument(gomock.Any(), elasticsearchMigrationIndex, "1", gomock.Any()).
		Return(assert.AnError)

	ds := elasticsearchDS{client: mockElasticsearch}
	mg := elasticsearchMigrator{elasticsearchDS: ds, migrator: &Datasource{}}

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
	}

	err := mg.commitMigration(mockContainer, data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to record migration")
}

func TestMigrationRunElasticsearchSuccess(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		ctx := context.Background()

		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				// Create an index
				settings := map[string]any{
					"settings": map[string]any{
						"number_of_shards": 1,
					},
				}
				err := d.Elasticsearch.CreateIndex(ctx, "test-index", settings)
				if err != nil {
					return err
				}

				// Index a document
				document := map[string]any{
					"title":   "Test Document",
					"content": "This is a test document",
				}
				err = d.Elasticsearch.IndexDocument(ctx, "test-index", "1", document)
				if err != nil {
					return err
				}

				d.Logger.Infof("Elasticsearch Migration Ran Successfully")

				return nil
			}},
		}

		mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

		// 1. checkAndCreateMigrationTable: Check if index exists (it doesn't)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError).Times(1)

		// 2. checkAndCreateMigrationTable: Create the migration index
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil).Times(1)

		// 3. Optimistic pre-check: Get last migration (called ONCE)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil).Times(1)

		// 4. Execute migration: Create test index
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), "test-index", gomock.Any()).
			Return(nil).Times(1)

		// 5. Execute migration: Index test document
		mockElasticsearch.EXPECT().IndexDocument(gomock.Any(), "test-index", "1", gomock.Any()).
			Return(nil).Times(1)

		// 6. Commit migration: Record migration in tracking index
		mockElasticsearch.EXPECT().IndexDocument(gomock.Any(), elasticsearchMigrationIndex, "1", gomock.Any()).
			Return(nil).Times(1)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "running migration")
	assert.Contains(t, logs, "Elasticsearch Migration Ran Successfully")
}

func TestMigrationRunElasticsearchMigrationFailure(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		ctx := context.Background()

		mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				err := d.Elasticsearch.CreateIndex(ctx, "test-index", map[string]any{})
				if err != nil {
					return err
				}

				return nil
			}},
		}

		// 1. checkAndCreateMigrationTable: Check if index exists (it doesn't)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError).Times(1)

		// 2. checkAndCreateMigrationTable: Create the migration index
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil).Times(1)

		// 3. Optimistic pre-check: Get last migration
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil).Times(1)

		// 4. Execute migration: Create test index fails
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), "test-index", gomock.Any()).
			Return(assert.AnError).Times(1)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "failed to run migration")
}

func TestMigrationRunElasticsearchCommitError(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		ctx := context.Background()

		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				err := d.Elasticsearch.CreateIndex(ctx, "test-index", map[string]any{})
				if err != nil {
					return err
				}

				return nil
			}},
		}

		mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

		// 1. checkAndCreateMigrationTable: Check if index exists (it doesn't)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError).Times(1)

		// 2. checkAndCreateMigrationTable: Create the migration index
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil).Times(1)

		// 3. Optimistic pre-check: Get last migration
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil).Times(1)

		// 4. Execute migration: Create test index succeeds
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), "test-index", gomock.Any()).
			Return(nil).Times(1)

		// 5. Commit migration: Record migration fails
		mockElasticsearch.EXPECT().IndexDocument(gomock.Any(), elasticsearchMigrationIndex, "1", gomock.Any()).
			Return(assert.AnError).Times(1)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "failed to commit migration")
	assert.Contains(t, logs, "failed to record migration")
}
