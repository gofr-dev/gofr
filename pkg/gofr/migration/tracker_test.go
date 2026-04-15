package migration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	goRedis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestInstallTrackers_SetsUsedOnSQLAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSQL := NewMockSQL(ctrl)
	mockSQL.EXPECT().Exec(gomock.Any(), gomock.Any()).Return(nil, nil)

	ds := &Datasource{SQL: mockSQL}
	used := installTrackers(ds)

	assert.Empty(t, used, "no datasource should be marked used before any call")

	_, _ = ds.SQL.Exec("SELECT 1")

	assert.True(t, used[dsSQL], "SQL should be marked used after Exec call")
}

func TestInstallTrackers_SetsUsedOnRedisAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockRedis := NewMockRedis(ctrl)
	mockRedis.EXPECT().Get(gomock.Any(), "key").Return(&goRedis.StringCmd{})

	ds := &Datasource{Redis: mockRedis}
	used := installTrackers(ds)

	ds.Redis.Get(t.Context(), "key")

	assert.True(t, used[dsRedis], "Redis should be marked used after Get call")
}

func TestInstallTrackers_SetsUsedOnClickhouseAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockCH := NewMockClickhouse(ctrl)
	mockCH.EXPECT().Exec(gomock.Any(), gomock.Any()).Return(nil)

	ds := &Datasource{Clickhouse: mockCH}
	used := installTrackers(ds)

	_ = ds.Clickhouse.Exec(t.Context(), "SELECT 1")

	assert.True(t, used[dsClickhouse], "Clickhouse should be marked used after Exec call")
}

func TestInstallTrackers_SetsUsedOnCassandraAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockCass := NewMockCassandra(ctrl)
	mockCass.EXPECT().Exec(gomock.Any()).Return(nil)

	ds := &Datasource{Cassandra: mockCass}
	used := installTrackers(ds)

	_ = ds.Cassandra.Exec("SELECT 1")

	assert.True(t, used[dsCassandra], "Cassandra should be marked used after Exec call")
}

func TestInstallTrackers_SetsUsedOnMongoAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockMongo := NewMockMongo(ctrl)
	mockMongo.EXPECT().InsertOne(gomock.Any(), "coll", gomock.Any()).Return(nil, nil)

	ds := &Datasource{Mongo: mockMongo}
	used := installTrackers(ds)

	_, _ = ds.Mongo.InsertOne(t.Context(), "coll", map[string]any{"k": "v"})

	assert.True(t, used[dsMongo], "Mongo should be marked used after InsertOne call")
}

func TestInstallTrackers_SetsUsedOnDGraphAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockDG := NewMockDGraph(ctrl)
	mockDG.EXPECT().ApplySchema(gomock.Any(), gomock.Any()).Return(nil)

	ds := &Datasource{DGraph: mockDG}
	used := installTrackers(ds)

	_ = ds.DGraph.ApplySchema(t.Context(), "schema{}")

	assert.True(t, used[dsDGraph], "DGraph should be marked used after ApplySchema call")
}

func TestInstallTrackers_SetsUsedOnArangoDBAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockArango := NewMockArangoDB(ctrl)
	mockArango.EXPECT().CreateDB(gomock.Any(), "testdb").Return(nil)

	ds := &Datasource{ArangoDB: mockArango}
	used := installTrackers(ds)

	_ = ds.ArangoDB.CreateDB(t.Context(), "testdb")

	assert.True(t, used[dsArangoDB], "ArangoDB should be marked used after CreateDB call")
}

func TestInstallTrackers_SetsUsedOnSurrealDBAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSurreal := NewMockSurrealDB(ctrl)
	mockSurreal.EXPECT().Query(gomock.Any(), "SELECT 1", gomock.Any()).Return(nil, nil)

	ds := &Datasource{SurrealDB: mockSurreal}
	used := installTrackers(ds)

	_, _ = ds.SurrealDB.Query(t.Context(), "SELECT 1", nil)

	assert.True(t, used[dsSurrealDB], "SurrealDB should be marked used after Query call")
}

func TestInstallTrackers_SetsUsedOnScyllaDBAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockScylla := NewMockScyllaDB(ctrl)
	mockScylla.EXPECT().Exec(gomock.Any()).Return(nil)

	ds := &Datasource{ScyllaDB: mockScylla}
	used := installTrackers(ds)

	_ = ds.ScyllaDB.Exec("SELECT 1")

	assert.True(t, used[dsScyllaDB], "ScyllaDB should be marked used after Exec call")
}

func TestInstallTrackers_SetsUsedOnElasticsearchAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockES := NewMockElasticsearch(ctrl)
	mockES.EXPECT().CreateIndex(gomock.Any(), "idx", gomock.Any()).Return(nil)

	ds := &Datasource{Elasticsearch: mockES}
	used := installTrackers(ds)

	_ = ds.Elasticsearch.CreateIndex(t.Context(), "idx", nil)

	assert.True(t, used[dsElasticsearch], "Elasticsearch should be marked used after CreateIndex call")
}

func TestInstallTrackers_SetsUsedOnOpenTSDBAccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockTSDB := NewMockOpenTSDB(ctrl)
	mockTSDB.EXPECT().PutDataPoints(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	ds := &Datasource{OpenTSDB: mockTSDB}
	used := installTrackers(ds)

	_ = ds.OpenTSDB.PutDataPoints(t.Context(), nil, "", nil)

	assert.True(t, used[dsOpenTSDB], "OpenTSDB should be marked used after PutDataPoints call")
}

func TestInstallTrackers_UnusedDatasourcesNotMarked(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSQL := NewMockSQL(ctrl)
	mockRedis := NewMockRedis(ctrl)

	// Only SQL is called, Redis is not
	mockSQL.EXPECT().Exec(gomock.Any(), gomock.Any()).Return(nil, nil)

	ds := &Datasource{SQL: mockSQL, Redis: mockRedis}
	used := installTrackers(ds)

	_, _ = ds.SQL.Exec("CREATE TABLE t (id INT)")

	assert.True(t, used[dsSQL], "SQL should be marked used")
	assert.False(t, used[dsRedis], "Redis should NOT be marked used when no Redis method was called")
}

func TestInstallTrackers_NilDatasourcesSkipped(t *testing.T) {
	ds := &Datasource{}
	used := installTrackers(ds)

	assert.Empty(t, used, "no datasources should be marked when all fields are nil")
}

func TestTrackedSQL_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSQL := NewMockSQL(ctrl)
	used := make(map[string]bool)
	tracked := &trackedSQL{inner: mockSQL, used: used, key: dsSQL}

	ctx := t.Context()

	t.Run("Query", func(t *testing.T) {
		delete(used, dsSQL)
		mockSQL.EXPECT().Query("SELECT 1").Return((*sql.Rows)(nil), nil)

		rows, err := tracked.Query("SELECT 1")
		require.NoError(t, err)

		if rows != nil {
			require.NoError(t, rows.Err())
		}

		assert.True(t, used[dsSQL])
	})

	t.Run("QueryRow", func(t *testing.T) {
		delete(used, dsSQL)
		mockSQL.EXPECT().QueryRow("SELECT 1").Return((*sql.Row)(nil))

		tracked.QueryRow("SELECT 1")

		assert.True(t, used[dsSQL])
	})

	t.Run("QueryRowContext", func(t *testing.T) {
		delete(used, dsSQL)
		mockSQL.EXPECT().QueryRowContext(ctx, "SELECT 1").Return((*sql.Row)(nil))

		tracked.QueryRowContext(ctx, "SELECT 1")

		assert.True(t, used[dsSQL])
	})

	t.Run("Exec", func(t *testing.T) {
		delete(used, dsSQL)
		mockSQL.EXPECT().Exec("INSERT INTO t VALUES (1)").Return(nil, nil)

		_, _ = tracked.Exec("INSERT INTO t VALUES (1)")

		assert.True(t, used[dsSQL])
	})

	t.Run("ExecContext", func(t *testing.T) {
		delete(used, dsSQL)
		mockSQL.EXPECT().ExecContext(ctx, "INSERT INTO t VALUES (1)").Return(nil, nil)

		_, _ = tracked.ExecContext(ctx, "INSERT INTO t VALUES (1)")

		assert.True(t, used[dsSQL])
	})
}

func TestTrackedRedis_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockRedis := NewMockRedis(ctrl)
	used := make(map[string]bool)
	tracked := &trackedRedis{inner: mockRedis, used: used, key: dsRedis}

	ctx := t.Context()

	t.Run("Get", func(t *testing.T) {
		delete(used, dsRedis)
		mockRedis.EXPECT().Get(ctx, "k").Return(&goRedis.StringCmd{})

		tracked.Get(ctx, "k")

		assert.True(t, used[dsRedis])
	})

	t.Run("Set", func(t *testing.T) {
		delete(used, dsRedis)
		mockRedis.EXPECT().Set(ctx, "k", "v", time.Duration(0)).Return(&goRedis.StatusCmd{})

		tracked.Set(ctx, "k", "v", 0)

		assert.True(t, used[dsRedis])
	})

	t.Run("Del", func(t *testing.T) {
		delete(used, dsRedis)
		mockRedis.EXPECT().Del(ctx, "k").Return(&goRedis.IntCmd{})

		tracked.Del(ctx, "k")

		assert.True(t, used[dsRedis])
	})

	t.Run("Rename", func(t *testing.T) {
		delete(used, dsRedis)
		mockRedis.EXPECT().Rename(ctx, "k", "k2").Return(&goRedis.StatusCmd{})

		tracked.Rename(ctx, "k", "k2")

		assert.True(t, used[dsRedis])
	})
}

func TestTrackedSQL_DelegatesToInner(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSQL := NewMockSQL(ctrl)
	used := make(map[string]bool)
	tracked := &trackedSQL{inner: mockSQL, used: used, key: dsSQL}

	expectedRows := (*sql.Rows)(nil)
	expectedErr := sql.ErrNoRows

	mockSQL.EXPECT().Query("SELECT 1").Return(expectedRows, expectedErr)

	rows, err := tracked.Query("SELECT 1")

	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, expectedRows, rows)

	if rows != nil {
		require.NoError(t, rows.Err())
	}
}

func TestTrackedMongo_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockMongo := NewMockMongo(ctrl)
	used := make(map[string]bool)
	tracked := &trackedMongo{inner: mockMongo, used: used, key: dsMongo}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"Find", func() {
			mockMongo.EXPECT().Find(ctx, "c", gomock.Any(), gomock.Any()).Return(nil)
			_ = tracked.Find(ctx, "c", nil, nil)
		}},
		{"FindOne", func() {
			mockMongo.EXPECT().FindOne(ctx, "c", gomock.Any(), gomock.Any()).Return(nil)
			_ = tracked.FindOne(ctx, "c", nil, nil)
		}},
		{"InsertOne", func() {
			mockMongo.EXPECT().InsertOne(ctx, "c", gomock.Any()).Return(nil, nil)
			_, _ = tracked.InsertOne(ctx, "c", nil)
		}},
		{"InsertMany", func() {
			mockMongo.EXPECT().InsertMany(ctx, "c", gomock.Any()).Return(nil, nil)
			_, _ = tracked.InsertMany(ctx, "c", nil)
		}},
		{"DeleteOne", func() {
			mockMongo.EXPECT().DeleteOne(ctx, "c", gomock.Any()).Return(int64(0), nil)
			_, _ = tracked.DeleteOne(ctx, "c", nil)
		}},
		{"DeleteMany", func() {
			mockMongo.EXPECT().DeleteMany(ctx, "c", gomock.Any()).Return(int64(0), nil)
			_, _ = tracked.DeleteMany(ctx, "c", nil)
		}},
		{"UpdateByID", func() {
			mockMongo.EXPECT().UpdateByID(ctx, "c", gomock.Any(), gomock.Any()).Return(int64(0), nil)
			_, _ = tracked.UpdateByID(ctx, "c", nil, nil)
		}},
		{"UpdateOne", func() {
			mockMongo.EXPECT().UpdateOne(ctx, "c", gomock.Any(), gomock.Any()).Return(nil)
			_ = tracked.UpdateOne(ctx, "c", nil, nil)
		}},
		{"UpdateMany", func() {
			mockMongo.EXPECT().UpdateMany(ctx, "c", gomock.Any(), gomock.Any()).Return(int64(0), nil)
			_, _ = tracked.UpdateMany(ctx, "c", nil, nil)
		}},
		{"Drop", func() {
			mockMongo.EXPECT().Drop(ctx, "c").Return(nil)
			_ = tracked.Drop(ctx, "c")
		}},
		{"CreateCollection", func() {
			mockMongo.EXPECT().CreateCollection(ctx, "c").Return(nil)
			_ = tracked.CreateCollection(ctx, "c")
		}},
		{"StartSession", func() {
			mockMongo.EXPECT().StartSession().Return(nil, nil)
			_, _ = tracked.StartSession()
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsMongo)
			tc.call()
			assert.True(t, used[dsMongo], "%s should mark Mongo as used", tc.name)
		})
	}
}

func TestTrackedElasticsearch_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockES := NewMockElasticsearch(ctrl)
	used := make(map[string]bool)
	tracked := &trackedElasticsearch{inner: mockES, used: used, key: dsElasticsearch}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"CreateIndex", func() {
			mockES.EXPECT().CreateIndex(ctx, "idx", gomock.Any()).Return(nil)
			_ = tracked.CreateIndex(ctx, "idx", nil)
		}},
		{"DeleteIndex", func() {
			mockES.EXPECT().DeleteIndex(ctx, "idx").Return(nil)
			_ = tracked.DeleteIndex(ctx, "idx")
		}},
		{"IndexDocument", func() {
			mockES.EXPECT().IndexDocument(ctx, "idx", "1", gomock.Any()).Return(nil)
			_ = tracked.IndexDocument(ctx, "idx", "1", nil)
		}},
		{"GetDocument", func() {
			mockES.EXPECT().GetDocument(ctx, "idx", "1").Return(nil, nil)
			_, _ = tracked.GetDocument(ctx, "idx", "1")
		}},
		{"UpdateDocument", func() {
			mockES.EXPECT().UpdateDocument(ctx, "idx", "1", gomock.Any()).Return(nil)
			_ = tracked.UpdateDocument(ctx, "idx", "1", nil)
		}},
		{"DeleteDocument", func() {
			mockES.EXPECT().DeleteDocument(ctx, "idx", "1").Return(nil)
			_ = tracked.DeleteDocument(ctx, "idx", "1")
		}},
		{"Bulk", func() {
			mockES.EXPECT().Bulk(ctx, gomock.Any()).Return(nil, nil)
			_, _ = tracked.Bulk(ctx, nil)
		}},
		{"Search", func() {
			mockES.EXPECT().Search(ctx, gomock.Any(), gomock.Any()).Return(nil, nil)
			_, _ = tracked.Search(ctx, nil, nil)
		}},
		{"HealthCheck", func() {
			mockES.EXPECT().HealthCheck(ctx).Return(nil, nil)
			_, _ = tracked.HealthCheck(ctx)
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsElasticsearch)
			tc.call()
			assert.True(t, used[dsElasticsearch], "%s should mark Elasticsearch as used", tc.name)
		})
	}
}

func TestTrackedClickhouse_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockCH := NewMockClickhouse(ctrl)
	used := make(map[string]bool)
	tracked := &trackedClickhouse{inner: mockCH, used: used, key: dsClickhouse}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"Exec", func() {
			mockCH.EXPECT().Exec(ctx, "q").Return(nil)
			_ = tracked.Exec(ctx, "q")
		}},
		{"Select", func() {
			mockCH.EXPECT().Select(ctx, gomock.Any(), "q").Return(nil)
			_ = tracked.Select(ctx, nil, "q")
		}},
		{"AsyncInsert", func() {
			mockCH.EXPECT().AsyncInsert(ctx, "q", true).Return(nil)
			_ = tracked.AsyncInsert(ctx, "q", true)
		}},
		{"HealthCheck", func() {
			mockCH.EXPECT().HealthCheck(ctx).Return(nil, nil)
			_, _ = tracked.HealthCheck(ctx)
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsClickhouse)
			tc.call()
			assert.True(t, used[dsClickhouse], "%s should mark Clickhouse as used", tc.name)
		})
	}
}

func TestTrackedOracle_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockOracle := NewMockOracle(ctrl)
	used := make(map[string]bool)
	tracked := &trackedOracle{inner: mockOracle, used: used, key: dsOracle}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"Select", func() {
			mockOracle.EXPECT().Select(ctx, gomock.Any(), "q").Return(nil)
			_ = tracked.Select(ctx, nil, "q")
		}},
		{"Exec", func() {
			mockOracle.EXPECT().Exec(ctx, "q").Return(nil)
			_ = tracked.Exec(ctx, "q")
		}},
		{"Begin", func() {
			mockOracle.EXPECT().Begin().Return(nil, nil)
			_, _ = tracked.Begin()
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsOracle)
			tc.call()
			assert.True(t, used[dsOracle], "%s should mark Oracle as used", tc.name)
		})
	}
}

func TestTrackedCassandra_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockCass := NewMockCassandra(ctrl)
	used := make(map[string]bool)
	tracked := &trackedCassandra{inner: mockCass, used: used, key: dsCassandra}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"Exec", func() {
			mockCass.EXPECT().Exec("q").Return(nil)
			_ = tracked.Exec("q")
		}},
		{"NewBatch", func() {
			mockCass.EXPECT().NewBatch("b", 0).Return(nil)
			_ = tracked.NewBatch("b", 0)
		}},
		{"BatchQuery", func() {
			mockCass.EXPECT().BatchQuery("b", "q").Return(nil)
			_ = tracked.BatchQuery("b", "q")
		}},
		{"ExecuteBatch", func() {
			mockCass.EXPECT().ExecuteBatch("b").Return(nil)
			_ = tracked.ExecuteBatch("b")
		}},
		{"HealthCheck", func() {
			mockCass.EXPECT().HealthCheck(ctx).Return(nil, nil)
			_, _ = tracked.HealthCheck(ctx)
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsCassandra)
			tc.call()
			assert.True(t, used[dsCassandra], "%s should mark Cassandra as used", tc.name)
		})
	}
}

func TestTrackedArangoDB_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockArango := NewMockArangoDB(ctrl)
	used := make(map[string]bool)
	tracked := &trackedArangoDB{inner: mockArango, used: used, key: dsArangoDB}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"CreateDB", func() {
			mockArango.EXPECT().CreateDB(ctx, "db").Return(nil)
			_ = tracked.CreateDB(ctx, "db")
		}},
		{"DropDB", func() {
			mockArango.EXPECT().DropDB(ctx, "db").Return(nil)
			_ = tracked.DropDB(ctx, "db")
		}},
		{"CreateCollection", func() {
			mockArango.EXPECT().CreateCollection(ctx, "db", "c", false).Return(nil)
			_ = tracked.CreateCollection(ctx, "db", "c", false)
		}},
		{"DropCollection", func() {
			mockArango.EXPECT().DropCollection(ctx, "db", "c").Return(nil)
			_ = tracked.DropCollection(ctx, "db", "c")
		}},
		{"CreateGraph", func() {
			mockArango.EXPECT().CreateGraph(ctx, "db", "g", gomock.Any()).Return(nil)
			_ = tracked.CreateGraph(ctx, "db", "g", nil)
		}},
		{"DropGraph", func() {
			mockArango.EXPECT().DropGraph(ctx, "db", "g").Return(nil)
			_ = tracked.DropGraph(ctx, "db", "g")
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsArangoDB)
			tc.call()
			assert.True(t, used[dsArangoDB], "%s should mark ArangoDB as used", tc.name)
		})
	}
}

func TestTrackedSurrealDB_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSurreal := NewMockSurrealDB(ctrl)
	used := make(map[string]bool)
	tracked := &trackedSurrealDB{inner: mockSurreal, used: used, key: dsSurrealDB}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"Query", func() {
			mockSurreal.EXPECT().Query(ctx, "q", gomock.Any()).Return(nil, nil)
			_, _ = tracked.Query(ctx, "q", nil)
		}},
		{"CreateNamespace", func() {
			mockSurreal.EXPECT().CreateNamespace(ctx, "ns").Return(nil)
			_ = tracked.CreateNamespace(ctx, "ns")
		}},
		{"CreateDatabase", func() {
			mockSurreal.EXPECT().CreateDatabase(ctx, "db").Return(nil)
			_ = tracked.CreateDatabase(ctx, "db")
		}},
		{"DropNamespace", func() {
			mockSurreal.EXPECT().DropNamespace(ctx, "ns").Return(nil)
			_ = tracked.DropNamespace(ctx, "ns")
		}},
		{"DropDatabase", func() {
			mockSurreal.EXPECT().DropDatabase(ctx, "db").Return(nil)
			_ = tracked.DropDatabase(ctx, "db")
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsSurrealDB)
			tc.call()
			assert.True(t, used[dsSurrealDB], "%s should mark SurrealDB as used", tc.name)
		})
	}
}

func TestTrackedDGraph_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockDG := NewMockDGraph(ctrl)
	used := make(map[string]bool)
	tracked := &trackedDGraph{inner: mockDG, used: used, key: dsDGraph}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"ApplySchema", func() {
			mockDG.EXPECT().ApplySchema(ctx, "s").Return(nil)
			_ = tracked.ApplySchema(ctx, "s")
		}},
		{"AddOrUpdateField", func() {
			mockDG.EXPECT().AddOrUpdateField(ctx, "f", "string", "").Return(nil)
			_ = tracked.AddOrUpdateField(ctx, "f", "string", "")
		}},
		{"DropField", func() {
			mockDG.EXPECT().DropField(ctx, "f").Return(nil)
			_ = tracked.DropField(ctx, "f")
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsDGraph)
			tc.call()
			assert.True(t, used[dsDGraph], "%s should mark DGraph as used", tc.name)
		})
	}
}

func TestTrackedOpenTSDB_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockTSDB := NewMockOpenTSDB(ctrl)
	used := make(map[string]bool)
	tracked := &trackedOpenTSDB{inner: mockTSDB, used: used, key: dsOpenTSDB}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"PutDataPoints", func() {
			mockTSDB.EXPECT().PutDataPoints(ctx, gomock.Any(), "", gomock.Any()).Return(nil)
			_ = tracked.PutDataPoints(ctx, nil, "", nil)
		}},
		{"PostAnnotation", func() {
			mockTSDB.EXPECT().PostAnnotation(ctx, gomock.Any(), gomock.Any()).Return(nil)
			_ = tracked.PostAnnotation(ctx, nil, nil)
		}},
		{"PutAnnotation", func() {
			mockTSDB.EXPECT().PutAnnotation(ctx, gomock.Any(), gomock.Any()).Return(nil)
			_ = tracked.PutAnnotation(ctx, nil, nil)
		}},
		{"DeleteAnnotation", func() {
			mockTSDB.EXPECT().DeleteAnnotation(ctx, gomock.Any(), gomock.Any()).Return(nil)
			_ = tracked.DeleteAnnotation(ctx, nil, nil)
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsOpenTSDB)
			tc.call()
			assert.True(t, used[dsOpenTSDB], "%s should mark OpenTSDB as used", tc.name)
		})
	}
}

func TestTrackedScyllaDB_AllMethodsMarkUsed(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockScylla := NewMockScyllaDB(ctrl)
	used := make(map[string]bool)
	tracked := &trackedScyllaDB{inner: mockScylla, used: used, key: dsScyllaDB}

	ctx := context.Background()

	methodTests := []struct {
		name string
		call func()
	}{
		{"Query", func() {
			mockScylla.EXPECT().Query(gomock.Any(), "SELECT 1").Return(nil)
			_ = tracked.Query(nil, "SELECT 1")
		}},
		{"QueryWithCtx", func() {
			mockScylla.EXPECT().QueryWithCtx(ctx, gomock.Any(), "SELECT 1").Return(nil)
			_ = tracked.QueryWithCtx(ctx, nil, "SELECT 1")
		}},
		{"Exec", func() {
			mockScylla.EXPECT().Exec("INSERT").Return(nil)
			_ = tracked.Exec("INSERT")
		}},
		{"ExecWithCtx", func() {
			mockScylla.EXPECT().ExecWithCtx(ctx, "INSERT").Return(nil)
			_ = tracked.ExecWithCtx(ctx, "INSERT")
		}},
		{"ExecCAS", func() {
			mockScylla.EXPECT().ExecCAS(gomock.Any(), "INSERT").Return(false, nil)
			_, _ = tracked.ExecCAS(nil, "INSERT")
		}},
		{"NewBatch", func() {
			mockScylla.EXPECT().NewBatch("b", 0).Return(nil)
			_ = tracked.NewBatch("b", 0)
		}},
		{"NewBatchWithCtx", func() {
			mockScylla.EXPECT().NewBatchWithCtx(ctx, "b", 0).Return(nil)
			_ = tracked.NewBatchWithCtx(ctx, "b", 0)
		}},
		{"BatchQuery", func() {
			mockScylla.EXPECT().BatchQuery("b", "INSERT").Return(nil)
			_ = tracked.BatchQuery("b", "INSERT")
		}},
		{"BatchQueryWithCtx", func() {
			mockScylla.EXPECT().BatchQueryWithCtx(ctx, "b", "INSERT").Return(nil)
			_ = tracked.BatchQueryWithCtx(ctx, "b", "INSERT")
		}},
		{"ExecuteBatchWithCtx", func() {
			mockScylla.EXPECT().ExecuteBatchWithCtx(ctx, "b").Return(nil)
			_ = tracked.ExecuteBatchWithCtx(ctx, "b")
		}},
	}

	for _, tc := range methodTests {
		t.Run(tc.name, func(t *testing.T) {
			delete(used, dsScyllaDB)
			tc.call()
			assert.True(t, used[dsScyllaDB], "%s should mark ScyllaDB as used", tc.name)
		})
	}
}
