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

		// Mock the migration index check (index doesn't exist initially)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// Mock the migration index creation
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil)

		// Mock the last migration query (no migrations yet) — called twice: pre-check + under lock
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getElasticsearchMigrationVersionsQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil).Times(2)

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

		// Mock the migration index check (index doesn't exist initially)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// Mock the migration index creation
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil)

		// Mock the last migration query (no migrations yet) — called twice: pre-check + under lock
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getElasticsearchMigrationVersionsQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil).Times(2)

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

		// checkAndCreateMigrationTable: Check if index exists (error)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// checkAndCreateMigrationTable: Try to create index (fails)
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(assert.AnError)

		Run(migrationMap, mockContainer)
	})

	assert.True(t, mockElasticsearch.ctrl.Satisfied())
}

func TestMigrationRunElasticsearchCurrentMigrationEqualLastMigration(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migrationMap := map[int64]Migrate{
			1: {UP: func(_ Datasource) error {
				return nil
			}},
		}

		mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)

		// Mock the migration index check (index doesn't exist initially)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// Mock the migration index creation
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil)

		// Mock the last migration query (migration 1 already exists)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getElasticsearchMigrationVersionsQuery()).
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
	})

	assert.Contains(t, logs, "no new migrations to run")
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

		// Mock the migration index check (index doesn't exist initially)
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
			Return(nil, assert.AnError)

		// Mock the migration index creation
		mockElasticsearch.EXPECT().CreateIndex(gomock.Any(), elasticsearchMigrationIndex, gomock.Any()).
			Return(nil)

		// Mock the last migration query (no migrations yet) — called twice: pre-check + under lock
		mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
			getElasticsearchMigrationVersionsQuery()).
			Return(map[string]any{
				"hits": map[string]any{
					"hits": []any{},
				},
			}, nil).Times(2)

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

func TestElasticsearchMigrator_getAppliedMigrations(t *testing.T) {
	hit := func(v float64) any {
		return map[string]any{"_source": map[string]any{"version": v}}
	}

	testCases := []struct {
		desc      string
		result    map[string]any
		searchErr error
		skipInner bool
		inner     map[int64]struct{}
		innerErr  error
		expected  map[int64]struct{}
		expErr    error
	}{
		{
			desc:     "backend versions merged with inner versions",
			result:   map[string]any{"hits": map[string]any{"hits": []any{hit(4), hit(3), hit(1)}}},
			inner:    map[int64]struct{}{2: {}},
			expected: map[int64]struct{}{1: {}, 2: {}, 3: {}, 4: {}},
		},
		{
			desc:     "no hits returns only inner set",
			result:   map[string]any{"hits": map[string]any{"hits": []any{}}},
			inner:    map[int64]struct{}{2: {}},
			expected: map[int64]struct{}{2: {}},
		},
		{
			desc:      "search error",
			result:    map[string]any{},
			searchErr: assert.AnError,
			skipInner: true,
			expErr:    assert.AnError,
		},
		{
			desc:     "inner migrator error",
			result:   map[string]any{"hits": map[string]any{"hits": []any{hit(1)}}},
			innerErr: assert.AnError,
			expErr:   assert.AnError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)
			mockMigrator := NewMockmigrator(ctrl)

			mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex},
				getElasticsearchMigrationVersionsQuery()).Return(tc.result, tc.searchErr)

			if !tc.skipInner {
				mockMigrator.EXPECT().getAppliedMigrations(mockContainer).Return(tc.inner, tc.innerErr)
			}

			mg := elasticsearchMigrator{elasticsearchDS: elasticsearchDS{client: mockElasticsearch}, migrator: mockMigrator}

			applied, err := mg.getAppliedMigrations(mockContainer)

			if tc.expErr != nil {
				require.ErrorIs(t, err, tc.expErr)
				assert.Nil(t, applied)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, applied)
		})
	}
}

// versionsPage answers a search over migration documents holding versions 1..total, highest first,
// honoring the query's search_after cursor and returning at most size hits.
func versionsPage(t *testing.T, query map[string]any, total, size int) map[string]any {
	t.Helper()

	start := 0

	if after, found := query["search_after"].([]any); found {
		require.Len(t, after, 1)

		last, ok := after[0].(int64)
		require.True(t, ok)

		start = total - int(last)
	}

	hits := []any{}
	for i := start; i < total && len(hits) < size; i++ {
		hits = append(hits, map[string]any{
			"_source": map[string]any{"version": float64(total - i)},
			"sort":    []any{int64(total - i)},
		})
	}

	return map[string]any{"hits": map[string]any{"hits": hits}}
}

func TestElasticsearchMigrator_getAppliedMigrations_Paging(t *testing.T) {
	testCases := []struct {
		desc string
		// pages is the number of full pages of migration documents, extra the documents past them.
		pages, extra int
		// failOnCall is the 1-based search call that errors, 0 for none.
		failOnCall int
		expCalls   int
		expErr     error
	}{
		{desc: "fewer documents than one page", extra: 3, expCalls: 1},
		{desc: "no documents", expCalls: 1},
		{desc: "documents spread over two pages", pages: 1, extra: 3, expCalls: 2},
		{desc: "documents spread over several pages", pages: 3, extra: 7, expCalls: 4},
		{desc: "exact multiple of the page size ends on an empty page", pages: 2, expCalls: 3},
		{desc: "search error on a later page", pages: 2, extra: 1, failOnCall: 2, expCalls: 2, expErr: assert.AnError},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockElasticsearch, mockContainer := initializeElasticsearchRunMocks(t)
			mockMigrator := NewMockmigrator(ctrl)

			var (
				calls int
				total int
			)

			// Documents hold versions 1..total and are served highest first, honoring size and search_after.
			mockElasticsearch.EXPECT().Search(gomock.Any(), []string{elasticsearchMigrationIndex}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ []string, query map[string]any) (map[string]any, error) {
					calls++

					size, ok := query["size"].(int)
					require.True(t, ok)
					assert.LessOrEqual(t, size, 1000, "page size must stay well under the default result window")

					if calls == 1 {
						total = tc.pages*size + tc.extra
					}

					if calls == tc.failOnCall {
						return nil, assert.AnError
					}

					return versionsPage(t, query, total, size), nil
				}).AnyTimes()

			if tc.expErr == nil {
				mockMigrator.EXPECT().getAppliedMigrations(mockContainer).Return(map[int64]struct{}{}, nil)
			}

			mg := elasticsearchMigrator{elasticsearchDS: elasticsearchDS{client: mockElasticsearch}, migrator: mockMigrator}

			applied, err := mg.getAppliedMigrations(mockContainer)

			assert.Equal(t, tc.expCalls, calls)

			if tc.expErr != nil {
				require.ErrorIs(t, err, tc.expErr)
				assert.Nil(t, applied)

				return
			}

			require.NoError(t, err)
			assert.Len(t, applied, total)

			for v := int64(1); v <= int64(total); v++ {
				if _, found := applied[v]; !found {
					t.Fatalf("version %d missing from the applied set", v)
				}
			}
		})
	}
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
		UsedDatasources: map[string]bool{dsElasticsearch: true},
	}

	err := mg.commitMigration(mockContainer, &data)
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
		UsedDatasources: map[string]bool{dsElasticsearch: true},
	}

	err := mg.commitMigration(mockContainer, &data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to record migration")
}

func TestElasticsearchMigrator_commitMigration_SkipsWhenNotUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockES := NewMockElasticsearch(ctrl)
	mockMigrator := NewMockmigrator(ctrl)

	m := elasticsearchMigrator{
		elasticsearchDS: elasticsearchDS{client: mockES},
		migrator:        mockMigrator,
	}

	c, _ := container.NewMockContainer(t)

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
		UsedDatasources: map[string]bool{},
	}

	mockMigrator.EXPECT().commitMigration(c, &data).Return(nil)

	err := m.commitMigration(c, &data)
	assert.NoError(t, err)
}

func TestExtractMigrationVersions(t *testing.T) {
	testCases := []struct {
		desc      string
		result    map[string]any
		expOutput []int64
	}{
		{desc: "missing hits", result: map[string]any{}, expOutput: nil},
		{desc: "hits list not a slice", result: map[string]any{"hits": map[string]any{"hits": "bad"}}, expOutput: nil},
		{desc: "hit not a map is skipped", result: map[string]any{"hits": map[string]any{"hits": []any{"bad"}}}, expOutput: []int64{}},
		{
			desc:      "source not a map is skipped",
			result:    map[string]any{"hits": map[string]any{"hits": []any{map[string]any{"_source": "bad"}}}},
			expOutput: []int64{},
		},
		{
			desc: "version not a number is skipped",
			result: map[string]any{"hits": map[string]any{"hits": []any{
				map[string]any{"_source": map[string]any{"version": "5"}},
			}}},
			expOutput: []int64{},
		},
		{
			desc: "valid versions are all returned, bad hits skipped",
			result: map[string]any{"hits": map[string]any{"hits": []any{
				map[string]any{"_source": map[string]any{"version": float64(12)}},
				"bad",
				map[string]any{"_source": map[string]any{"version": float64(7)}},
			}}},
			expOutput: []int64{12, 7},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			versions, _ := extractMigrationVersions(tc.result)

			assert.Equal(t, tc.expOutput, versions)
		})
	}
}

func TestElasticsearchMigrator_Delegation(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockContainer, _ := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)
	mockLogger := container.NewMockLogger(ctrl)
	mockContainer.Logger = mockLogger

	m := elasticsearchMigrator{migrator: mockMigrator}
	data := transactionData{MigrationNumber: 4}

	mockMigrator.EXPECT().rollback(mockContainer, &data)
	mockLogger.EXPECT().Fatalf("Migration %v failed.", int64(4))
	mockMigrator.EXPECT().lock(gomock.Any(), gomock.Any(), mockContainer, "owner-1").Return(assert.AnError)
	mockMigrator.EXPECT().unlock(mockContainer, "owner-1").Return(assert.AnError)

	m.rollback(mockContainer, &data)

	require.ErrorIs(t, m.lock(t.Context(), func() {}, mockContainer, "owner-1"), assert.AnError)
	require.ErrorIs(t, m.unlock(mockContainer, "owner-1"), assert.AnError)
	assert.Equal(t, "Elasticsearch", m.name())
}
