package migration

import (
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
	mockContainer.Logger = logging.NewMockLogger(logging.DEBUG)
	mockContainer.Elasticsearch = mockElasticsearch

	return mockElasticsearch, mockContainer
}

func TestMigrationRunElasticsearchSuccess(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				// Create an index
				settings := map[string]any{
					"settings": map[string]any{
						"number_of_shards": 1,
					},
				}
				err := d.Elasticsearch.CreateIndex(t.Context(), "test-index", settings)
				if err != nil {
					return err
				}

				// Index a document
				document := map[string]any{
					"title":   "Test Document",
					"content": "This is a test document",
				}
				err = d.Elasticsearch.IndexDocument(t.Context(), "test-index", "1", document)
				if err != nil {
					return err
				}

				d.Logger.Infof("Elasticsearch Migration Ran Successfully")

				return nil
			}},
		}

		mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

		// Pre-check
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil)

		// Mock the migration index check (index doesn't exist initially)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// Mock the migration index creation
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil)

		// Re-fetch after lock
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil)

		// Mock the migration operations
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), "test-index", gomock.Any()).
			Return(nil)
		mockElasticsearch.EXPECT().IndexDocument(gomock.Any(), "test-index", "1", gomock.Any()).
			Return(nil)

		// Mock the migration record insertion
		mockElasticsearch.EXPECT().IndexDocument(gomock.Any(), elasticsearchMigrationIndex, "1", gomock.Any()).
			Return(nil)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "Migration 1 ran successfully")
	assert.Contains(t, logs, "Elasticsearch Migration Ran Successfully")
}

func TestMigrationRunElasticsearchMigrationFailure(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				err := d.Elasticsearch.CreateIndex(t.Context(), "test-index", map[string]any{})
				if err != nil {
					return err
				}

				return nil
			}},
		}

		// Pre-check
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil)

		// Mock the migration index check (index doesn't exist initially)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// Mock the migration index creation
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil)

		// Re-fetch after lock
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil)

		// Mock the migration operation failure
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), "test-index", gomock.Any()).
			Return(assert.AnError)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "failed to run migration : [1], err: assert.AnError general error for testing")
}

func TestMigrationRunElasticsearchMigrationFailureWhileCheckingTable(t *testing.T) {
	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	testutil.StderrOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			1: {UP: func(_ Datasource) error {
				return nil
			}},
		}

		// checkAndCreateMigrationTable is called first
		// Mock the migration index check failure
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// Mock the migration index creation failure
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(assert.AnError)

		Run(migrationMap, mockContainer)
	})

	assert.True(t, mockElasticsearch.ctrl.Satisfied())
}

func TestMigrationRunElasticsearchCurrentMigrationEqualLastMigration(t *testing.T) {
	migrationMap := map[int64]Migrate{
		1: {UP: func(_ Datasource) error {
			return nil
		}},
	}

	mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

	// checkAndCreateMigrationTable is called first
	mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
		gomock.Any()).
		Return(map[string]any{
			"hits": map[string]any{
				"hits": []any{},
			},
		}, nil)

	// Then getLastMigration is called - returns 1, so migration 1 is skipped
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

func TestMigrationRunElasticsearchCommitError(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			1: {UP: func(d Datasource) error {
				err := d.Elasticsearch.CreateIndex(t.Context(), "test-index", map[string]any{})
				if err != nil {
					return err
				}

				return nil
			}},
		}

		mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

		// Pre-check
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil)

		// Mock the migration index check (index doesn't exist initially)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// Mock the migration index creation
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil)

		// Re-fetch after lock
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getLastElasticsearchMigrationQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil)

		// Mock the migration operation success
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), "test-index", gomock.Any()).
			Return(nil)

		// Mock the migration record insertion failure
		mockElasticsearch.EXPECT().IndexDocument(gomock.Any(), elasticsearchMigrationIndex, "1", gomock.Any()).
			Return(assert.AnError)

		Run(migrationMap, mockContainer)
	})

	assert.Contains(t, logs, "failed to commit migration, err: failed to record migration: assert.AnError general error for testing")
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

	lastMigration := mg.getLastMigration(mockContainer)
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

	lastMigration := mg.getLastMigration(mockContainer)
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
